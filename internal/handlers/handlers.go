package handlers

import (
	"cms-go/internal/db"
	"cms-go/internal/models"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
)

func DynamicPage(c echo.Context) error {
	path := c.Request().URL.Path

	// 1) load page
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

	// 3) parse layout.Structure (JSON)
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(layout.Structure), &schema); err != nil {
		return c.String(http.StatusInternalServerError, "layout parse error: "+err.Error())
	}

	// 4) inject page.Content into "content" component
	if rows, ok := schema["rows"].([]interface{}); ok {
		for _, r := range rows {
			row, _ := r.(map[string]interface{})
			if cols, ok := row["columns"].([]interface{}); ok {
				for _, c := range cols {
					col, _ := c.(map[string]interface{})
					if comps, ok := col["components"].([]interface{}); ok {
						for _, cmp := range comps {
							comp, _ := cmp.(map[string]interface{})
							if comp["type"] == "content" {
								props, _ := comp["props"].(map[string]interface{})
								if props == nil {
									props = map[string]interface{}{}
								}
								props["body"] = page.Content
								comp["props"] = props
							}
						}
					}
				}
			}
		}
	}

	// 5) render with layout template
	data := map[string]interface{}{
		"Title": page.Title,
		"rows":  schema["rows"],
	}

	return c.Render(http.StatusOK, "front-layout", data)
}

func AdminDashboard(c echo.Context) error {
	return c.String(http.StatusOK, "Admin Dashboard")
}

func AdminPages(c echo.Context) error {
	var pages []models.Page
	db.DB.Find(&pages)

	return c.Render(http.StatusOK, "pages.html", map[string]interface{}{
		"Pages": pages,
	})
}

func AdminCreatePage(c echo.Context) error {
	title := c.FormValue("title")
	slug := c.FormValue("slug")
	content := c.FormValue("content")
	page := models.Page{Title: title, Slug: slug, Content: content, Type: "page"}

	db.DB.Create(&page)
	return c.Redirect(http.StatusFound, "/admin/pages")
}

func AdminMenus(c echo.Context) error {
	return c.String(http.StatusOK, "Menu management (DB version)")
}

// Layout Editor
func AdminLayouts(c echo.Context) error {
	var layouts []models.Layout
	db.DB.Find(&layouts)
	return c.Render(http.StatusOK, "admin/layouts.html", map[string]interface{}{
		"Layouts": layouts,
	})
}

func AdminCreateLayout(c echo.Context) error {
	name := c.FormValue("name")
	structure := c.FormValue("structure")
	layout := models.Layout{Name: name, Structure: structure}
	db.DB.Create(&layout)
	return c.Redirect(http.StatusFound, "/admin/layouts")
}

// Components List
func AdminComponents(c echo.Context) error {
	var comps []models.Component
	db.DB.Find(&comps)
	return c.Render(http.StatusOK, "admin/components.html", map[string]interface{}{
		"Components": comps,
	})
}

// Component Editor (New)
func AdminNewComponent(c echo.Context) error {
	return c.Render(http.StatusOK, "admin/component_form.html", nil)
}

func AdminCreateComponent(c echo.Context) error {
	comp := models.Component{
		Name:     c.FormValue("name"),
		Schema:   c.FormValue("schema"),
		Template: c.FormValue("template"),
	}
	db.DB.Create(&comp)
	return c.Redirect(http.StatusFound, "/admin/components")
}

// Component Editor (Edit)
func AdminEditComponent(c echo.Context) error {
	id := c.Param("id")
	var comp models.Component
	if err := db.DB.First(&comp, id).Error; err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	return c.Render(http.StatusOK, "admin/component_form.html", map[string]interface{}{
		"Component": comp,
	})
}

func AdminUpdateComponent(c echo.Context) error {
	id := c.Param("id")
	var comp models.Component
	if err := db.DB.First(&comp, id).Error; err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	comp.Name = c.FormValue("name")
	comp.Schema = c.FormValue("schema")
	comp.Template = c.FormValue("template")
	db.DB.Save(&comp)

	return c.Redirect(http.StatusFound, "/admin/components")
}
