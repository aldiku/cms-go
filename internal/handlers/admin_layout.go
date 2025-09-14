package handlers

import (
	"cms-go/internal/db"
	"cms-go/internal/models"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

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

	return renderWithLayout(c.Response().Writer, "internal/views/admin/admin-layout.html", "internal/views/admin/layouts.html", data)
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
	err := SaveLayoutToFile(layout)
	if err != nil {
		fmt.Println("Failed to save layout to file:", err)
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
		err := SaveLayoutToFile(layout)
		if err != nil {
			fmt.Println("Failed to save layout to file:", err)
		}

		return c.Redirect(http.StatusFound, "/admin/layouts")
	}

	// Render edit page

	data := map[string]interface{}{
		"Layout": layout,
	}

	return renderWithLayout(c.Response().Writer, "internal/views/admin/admin-layout.html", "internal/views/admin/edit_layout.html", data)
}

func SaveComponentToFile(comp models.Component) error {
	dir := "internal/views/generated/components"
	os.MkdirAll(dir, 0755)

	filePath := filepath.Join(dir, comp.Name+".html")

	content := fmt.Sprintf(`{{ define "%s" }}%s{{ end }}`, comp.Name, comp.Template)
	return os.WriteFile(filePath, []byte(content), 0644)
}

func SaveLayoutToFile(layout models.Layout) error {
	dir := "internal/views/generated/layouts"
	os.MkdirAll(dir, 0755)

	filePath := filepath.Join(dir, fmt.Sprintf("layout-%d.html", layout.ID))

	// Layout biasanya sudah berisi {{ define ... }} di DB
	return os.WriteFile(filePath, []byte(layout.Template), 0644)
}
