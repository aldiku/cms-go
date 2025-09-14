package server

import (
	"bytes"
	"cms-go/internal/db"
	"cms-go/internal/models"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// RenderComponent mencoba render component, skip kalau tidak ada template
func NewRenderer() *Template {
	root := template.New("").Funcs(template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"renderComponent": func(name string, data interface{}) template.HTML {
			return template.HTML("")
		},
	})

	// parse base views
	tmpl, err := root.ParseGlob("internal/views/**/*.html")
	if err != nil {
		panic(err)
	}

	// parse generated views
	tmpl, err = tmpl.ParseGlob("internal/views/generated/**/*.html")
	if err != nil {
		panic(err)
	}

	// reinject funcs (supaya renderComponent jalan)
	tmpl.Funcs(template.FuncMap{
		"renderComponent": func(name string, data interface{}) template.HTML {
			return RenderComponent(tmpl, name, data)
		},
	})

	return &Template{templates: tmpl}
}

func DynamicPage(c echo.Context) error {
	path := c.Request().URL.Path
	renderer := c.Echo().Renderer.(*Template)

	// 1) load page from DB
	var page models.Page
	if err := db.DB.Where("slug = ?", path).First(&page).Error; err != nil {
		return c.Render(http.StatusNotFound, "404.html", nil)
	}

	// 2) load layout
	var layout models.Layout
	if page.LayoutID == 0 {
		if err := db.DB.Where("name = ?", "front-layout").First(&layout).Error; err != nil {
			return c.Render(http.StatusNotFound, "404.html", nil)
		}
	} else {
		if err := db.DB.First(&layout, page.LayoutID).Error; err != nil {
			return c.Render(http.StatusNotFound, "404.html", nil)
		}
	}

	// 3) parse layout structure
	var layoutSchema map[string]interface{}
	if err := json.Unmarshal([]byte(layout.Structure), &layoutSchema); err != nil {
		return c.String(http.StatusInternalServerError, "layout parse error: "+err.Error())
	}

	// 4) parse page content JSON
	var pageSchema map[string]interface{}
	if err := json.Unmarshal([]byte(page.Content), &pageSchema); err != nil {
		return c.String(http.StatusInternalServerError, "page parse error: "+err.Error())
	}

	// 5) cari rows layout, lalu replace content.html
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

								// Ambil page.Content JSON → render ke HTML string
								// Asumsikan pageSchema punya "rows"
								if pageHTML, err := renderPageJSON(pageSchema, renderer); err == nil {
									props["html"] = template.HTML(pageHTML) // 🚨 langsung HTML
								}

								comp["props"] = props
							}
						}
					}
				}
			}
		}
	}

	// 6) data context
	data := map[string]interface{}{
		"Title": page.Title,
		"rows":  layoutSchema["rows"], // sudah di-modify
		"page":  page,
	}

	// 7) render dynamic
	return renderDynamic(c, layout, data, renderer)
}

// example RenderComponent: lookup subtemplate by name and execute
func RenderComponent(tmpl *template.Template, name string, data interface{}) template.HTML {
	var buf bytes.Buffer

	// data biasanya map[string]interface{}
	props, _ := data.(map[string]interface{})

	// kalau ada props["html"], pastikan ini langsung template.HTML
	if htmlContent, ok := props["html"]; ok {
		props["html"] = template.HTML(fmt.Sprintf("%v", htmlContent))
	}

	err := tmpl.ExecuteTemplate(&buf, name, props)
	if err != nil {
		fmt.Printf("renderComponent error: %v\n", err)
		return template.HTML("")
	}
	return template.HTML(buf.String())
}

func renderDynamic(c echo.Context, layout models.Layout, data interface{}, renderer *Template) error {
	// fresh root setiap request
	root := template.New("").Funcs(template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"renderComponent": func(name string, data interface{}) template.HTML {
			// nanti dipanggil setelah semua file ter-parse
			return template.HTML("")
		},
	})

	// parse semua base view dari folder internal/views
	tmpl, err := root.ParseGlob("internal/views/**/*.html")
	if err != nil {
		fmt.Println("parse base views error:", err)
		return c.Render(http.StatusOK, "front-layout", data)
	}

	// parse semua file dari generated
	tmpl, err = tmpl.ParseGlob("internal/views/generated/**/*.html")
	if err != nil {
		fmt.Println("parse generated views error:", err)
		return c.Render(http.StatusOK, "front-layout", data)
	}

	// reinject renderComponent supaya bisa panggil sub-template
	tmpl.Funcs(template.FuncMap{
		"renderComponent": func(name string, data interface{}) template.HTML {
			var buf bytes.Buffer
			if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
				fmt.Printf("renderComponent error: %v\n", err)
				return template.HTML("")
			}
			return template.HTML(buf.String())
		},
	})

	// kalau layout.Template masih ada di DB, parse juga
	if strings.TrimSpace(layout.Template) != "" {
		if _, err := tmpl.Parse(layout.Template); err != nil {
			fmt.Println("parse layout error:", err)
			return c.Render(http.StatusOK, "front-layout", data)
		}
	}

	// execute template sesuai layout
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
		// fallback kalau "base" tidak ada → coba langsung execute
		if err2 := tmpl.Execute(&buf, data); err2 != nil {
			fmt.Println("exec error:", err2)
			return c.Render(http.StatusOK, "front-layout", data)
		}
	}

	return c.HTML(http.StatusOK, buf.String())
}

func renderPageJSON(pageSchema map[string]interface{}, renderer *Template) (string, error) {
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

						html := string(RenderComponent(renderer.templates, typ, props))
						buf.WriteString(html)
					}
				}
				buf.WriteString(`</div>`)
			}
		}
		buf.WriteString(`</div>`)
	}

	return buf.String(), nil
}
