package handlers

import (
	"cms-go/internal/db"
	"cms-go/internal/models"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

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
	err := SaveComponentToFile(comp)
	if err != nil {
		fmt.Println("Failed to save layout to file:", err)
	}
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
	err := SaveComponentToFile(comp)
	if err != nil {
		fmt.Println("Failed to save layout to file:", err)
	}
	return c.Redirect(http.StatusFound, "/admin/components")
}
