// Package generator regenerates the on-disk views (component partials, layout
// partials, and fully-rendered page HTML) from the DB. Handlers call
// GenerateTemplatesFromDB whenever a page, layout, or component is created or
// updated, so the public site can serve requests straight from disk instead of
// hitting the DB or executing templates on every request.
package generator

import (
	"bytes"
	"cms-go/internal/config"
	"cms-go/internal/db"
	"cms-go/internal/models"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	baseDir   = "internal/views/generated"
	compDir   = baseDir + "/components"
	layoutDir = baseDir + "/layouts"
	pagesDir  = baseDir + "/pages"
)

// ParseTemplates parses every base view plus every generated component/layout
// partial from disk into a single template set.
func ParseTemplates() (*template.Template, error) {
	root := template.New("").Funcs(template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"renderComponent": func(name string, data interface{}) template.HTML {
			return template.HTML("")
		},
	})

	tmpl, err := root.ParseGlob("internal/views/**/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse base views: %w", err)
	}

	if generated, err := tmpl.ParseGlob("internal/views/generated/**/*.html"); err != nil {
		return nil, fmt.Errorf("parse generated views: %w", err)
	} else {
		tmpl = generated
	}

	tmpl.Funcs(template.FuncMap{
		"renderComponent": func(name string, data interface{}) template.HTML {
			return RenderComponent(tmpl, name, data)
		},
	})

	return tmpl, nil
}

// RenderComponent renders a page component. If props carries raw "html" (or a
// "path" to an HTML file), that content becomes the entire rendered output,
// (e.g. "hero") is looked up and executed with props as its data (e.g. {{.headline}}).
func RenderComponent(tmpl *template.Template, name string, data interface{}) template.HTML {
	props, _ := data.(map[string]interface{})

	if path, ok := props["path"].(string); ok && path != "" {
		content, err := os.ReadFile(config.RootPath() + path)
		if err != nil {
			log.Printf("Error reading HTML file %s: %v", path, err)
			return template.HTML("<!-- Error loading HTML file -->")
		}
		return template.HTML(content)
	}

	if raw, ok := props["html"]; ok {
		switch v := raw.(type) {
		case string:
			props["html"] = template.HTML(v)
		}
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, props); err != nil {
		log.Printf("renderComponent error for %q: %v", name, err)
		return template.HTML("")
	}
	return template.HTML(buf.String())
}

func renderPageComponents(pageSchema map[string]interface{}, tmpl *template.Template) string {
	var buf strings.Builder

	rows, _ := pageSchema["rows"].([]interface{})
	for _, r := range rows {
		row, _ := r.(map[string]interface{})
		buf.WriteString(`<div class="row">`)
		if cols, ok := row["columns"].([]interface{}); ok {
			for _, colRaw := range cols {
				col, _ := colRaw.(map[string]interface{})
				buf.WriteString(`<div class="col">`)
				if comps, ok := col["components"].([]interface{}); ok {
					for _, compRaw := range comps {
						comp, _ := compRaw.(map[string]interface{})
						typ, _ := comp["type"].(string)
						props, _ := comp["props"].(map[string]interface{})
						buf.WriteString(string(RenderComponent(tmpl, typ, props)))
					}
				}
				buf.WriteString(`</div>`)
			}
		}
		buf.WriteString(`</div>`)
	}

	return buf.String()
}

// RenderPage renders a single page with its layout to a final HTML string.
// tmpl is cloned internally so callers can safely reuse the same base template
// set across many pages: html/template forbids Parse-ing a tree again once any
// part of it has been Execute-d, which would otherwise break after the first page.
func RenderPage(base *template.Template, page models.Page, layout models.Layout) (string, error) {
	tmpl, err := base.Clone()
	if err != nil {
		return "", fmt.Errorf("clone template: %w", err)
	}

	// layout.Template harus di-parse sebelum ada template lain yang di-execute
	if strings.TrimSpace(layout.Template) != "" {
		if _, err := tmpl.Parse(layout.Template); err != nil {
			return "", fmt.Errorf("parse layout template: %w", err)
		}
		// re-inject renderComponent supaya menunjuk ke clone ini, bukan base
		tmpl.Funcs(template.FuncMap{
			"renderComponent": func(name string, data interface{}) template.HTML {
				return RenderComponent(tmpl, name, data)
			},
		})
	}

	var layoutSchema map[string]interface{}
	if err := json.Unmarshal([]byte(layout.Structure), &layoutSchema); err != nil {
		return "", fmt.Errorf("layout parse error: %w", err)
	}

	// Determine what HTML should be rendered into the content component.
	var renderedHTML template.HTML

	switch page.Type {
	case "html":
		// Raw HTML stored directly in page.Content
		renderedHTML = template.HTML(page.Content)

	default: // page, post
		var pageSchema map[string]interface{}
		if err := json.Unmarshal([]byte(page.Content), &pageSchema); err != nil {
			return "", fmt.Errorf("page parse error: %w", err)
		}

		renderedHTML = template.HTML(renderPageComponents(pageSchema, tmpl))
	}

	// Replace the content component with the rendered HTML.
	if rows, ok := layoutSchema["rows"].([]interface{}); ok {
		for _, r := range rows {
			row, _ := r.(map[string]interface{})
			if cols, ok := row["columns"].([]interface{}); ok {
				for _, colRaw := range cols {
					col, _ := colRaw.(map[string]interface{})
					if comps, ok := col["components"].([]interface{}); ok {
						for _, compRaw := range comps {
							comp, _ := compRaw.(map[string]interface{})
							if comp["type"] == "content" {
								props, _ := comp["props"].(map[string]interface{})
								if props == nil {
									props = map[string]interface{}{}
								}
								props["html"] = renderedHTML
								comp["props"] = props
							}
						}
					}
				}
			}
		}
	}

	data := map[string]interface{}{
		"Title":   page.Title,
		"rows":    layoutSchema["rows"],
		"page":    page,
		"seoHead": BuildSEOHead(page, config.SiteURL()),
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
		buf.Reset()
		if err2 := tmpl.Execute(&buf, data); err2 != nil {
			return "", fmt.Errorf("execute template: %w", err2)
		}
	}

	return buf.String(), nil
}

// PageFilePath returns the on-disk path where a page's pre-rendered HTML for
// the given request path/slug is (or would be) stored.
func PageFilePath(slug string) string {
	clean := strings.TrimPrefix(slug, "/")
	if clean == "" {
		clean = "index"
	}
	return filepath.Join(pagesDir, clean+".html")
}

// GenerateTemplatesFromDB regenerates component partials, layout partials, and
// every page's fully-rendered HTML from the DB onto disk.
func GenerateTemplatesFromDB() error {
	if err := os.MkdirAll(compDir, 0755); err != nil {
		return fmt.Errorf("failed to create components dir: %w", err)
	}
	if err := os.MkdirAll(layoutDir, 0755); err != nil {
		return fmt.Errorf("failed to create layouts dir: %w", err)
	}
	if err := os.MkdirAll(pagesDir, 0755); err != nil {
		return fmt.Errorf("failed to create pages dir: %w", err)
	}

	// --- Components ---
	var comps []models.Component
	if err := db.DB.Find(&comps).Error; err != nil {
		return fmt.Errorf("fetch components: %w", err)
	}
	for _, comp := range comps {
		if comp.Template == "" {
			continue
		}
		filePath := filepath.Join(compDir, comp.Name+".html")
		content := fmt.Sprintf(`{{ define "%s" }}%s{{ end }}`, comp.Name, comp.Template)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			log.Printf("failed to write component %s: %v", comp.Name, err)
		}
	}

	// --- Layouts ---
	var layouts []models.Layout
	if err := db.DB.Find(&layouts).Error; err != nil {
		return fmt.Errorf("fetch layouts: %w", err)
	}
	for _, layout := range layouts {
		if layout.Template == "" {
			continue
		}
		filePath := filepath.Join(layoutDir, fmt.Sprintf("layout-%d.html", layout.ID))
		if err := os.WriteFile(filePath, []byte(layout.Template), 0644); err != nil {
			log.Printf("failed to write layout %d: %v", layout.ID, err)
		}
	}

	// --- Pages: render the N latest-updated ones and cache the HTML to disk.
	// The pages dir is wiped first so nothing stale survives a layout or
	// component change (or a slug rename); evicted pages regenerate lazily
	// on first request via GeneratePage.
	if err := os.RemoveAll(pagesDir); err != nil {
		return fmt.Errorf("failed to clear pages dir: %w", err)
	}
	if err := os.MkdirAll(pagesDir, 0755); err != nil {
		return fmt.Errorf("failed to recreate pages dir: %w", err)
	}

	tmpl, err := ParseTemplates()
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}

	layoutsByID := make(map[uint]models.Layout, len(layouts))
	var defaultLayout models.Layout
	for _, l := range layouts {
		layoutsByID[l.ID] = l
		if l.Name == "front-layout" {
			defaultLayout = l
		}
	}

	var pages []models.Page
	if err := db.DB.Order("updated_at DESC").Limit(config.GeneratePageLimit()).Find(&pages).Error; err != nil {
		return fmt.Errorf("fetch pages: %w", err)
	}
	for _, page := range pages {
		layout := defaultLayout
		if page.LayoutID != 0 {
			if l, ok := layoutsByID[page.LayoutID]; ok {
				layout = l
			}
		}

		html, err := RenderPage(tmpl, page, layout)
		if err != nil {
			log.Printf("failed to render page %s: %v", page.Slug, err)
			continue
		}

		filePath := PageFilePath(page.Slug)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			log.Printf("failed to create dir for page %s: %v", page.Slug, err)
			continue
		}
		if err := os.WriteFile(filePath, []byte(html), 0644); err != nil {
			log.Printf("failed to write page %s: %v", page.Slug, err)
		}
	}

	log.Println("✅ Generated templates + pages from DB into", baseDir)
	return nil
}

// GeneratePage renders the single page whose slug matches the request path,
// writes it to disk so later requests are served straight from the file, and
// returns the HTML. Returns gorm.ErrRecordNotFound when no page matches.
// Used by the public handler as the cache-miss path for pages outside the
// pre-rendered top-N set.
func GeneratePage(path string) ([]byte, error) {
	var page models.Page
	// Slugs are stored with a leading slash ("/about-us"), but match the
	// trimmed form too in case older rows were saved without it.
	if err := db.DB.Where("slug = ? OR slug = ?", path, strings.TrimPrefix(path, "/")).First(&page).Error; err != nil {
		return nil, err
	}

	var layout models.Layout
	if page.LayoutID != 0 {
		if err := db.DB.First(&layout, page.LayoutID).Error; err != nil {
			return nil, fmt.Errorf("fetch layout %d: %w", page.LayoutID, err)
		}
	} else if err := db.DB.Where("name = ?", "front-layout").First(&layout).Error; err != nil {
		return nil, fmt.Errorf("fetch default layout: %w", err)
	}

	tmpl, err := ParseTemplates()
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	html, err := RenderPage(tmpl, page, layout)
	if err != nil {
		return nil, fmt.Errorf("render page %s: %w", page.Slug, err)
	}

	// Cache to disk best-effort: a write failure shouldn't fail the request.
	filePath := PageFilePath(page.Slug)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		log.Printf("failed to create dir for page %s: %v", page.Slug, err)
	} else if err := os.WriteFile(filePath, []byte(html), 0644); err != nil {
		log.Printf("failed to write page %s: %v", page.Slug, err)
	}

	return []byte(html), nil
}
