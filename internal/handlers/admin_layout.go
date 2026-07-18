package handlers

import (
	"cms-go/internal/db"
	"cms-go/internal/generator"
	"cms-go/internal/models"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Layout Editor
func AdminLayouts(c echo.Context) error {
	var layouts []models.Layout
	if err := db.DB.Find(&layouts).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load layouts")
	}

	data := map[string]interface{}{
		"Layouts": layouts,
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/layouts.html", data)
}

func AdminCreateLayout(c echo.Context) error {
	name := c.FormValue("name")
	structure := c.FormValue("structure")

	layout := models.Layout{
		Name:      name,
		Structure: structure,
	}

	if err := db.DB.Create(&layout).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to create layout")
	}
	if err := generator.GenerateTemplatesFromDB(); err != nil {
		fmt.Println("template generation error:", err)
	}

	return c.Redirect(http.StatusFound, "/admin/layouts")
}

func AdminEditLayout(c echo.Context) error {
	id := c.Param("id")

	var layout models.Layout
	if err := db.DB.First(&layout, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Layout not found")
	}

	if c.Request().Method == http.MethodPost {
		// Update layout with form values
		layout.Name = c.FormValue("name")
		layout.Structure = c.FormValue("structure")
		layout.Template = c.FormValue("template")

		if err := db.DB.Save(&layout).Error; err != nil {
			return c.String(http.StatusInternalServerError, "Failed to update layout")
		}
		if err := generator.GenerateTemplatesFromDB(); err != nil {
			fmt.Println("template generation error:", err)
		}

		return c.Redirect(http.StatusFound, "/admin/layouts")
	}

	// Render edit page

	data := map[string]interface{}{
		"Layout": layout,
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/edit_layout.html", data)
}
