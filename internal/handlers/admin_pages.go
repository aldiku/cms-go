package handlers

import (
	"cms-go/internal/db"
	"cms-go/internal/models"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

func AdminPages(c echo.Context) error {
	var pages []models.Page
	db.DB.Find(&pages)

	data := map[string]interface{}{
		"Pages": pages,
	}

	return renderWithLayout(c.Response().Writer, "internal/views/admin/admin-layout.html", "internal/views/admin/pages.html", data)
}

func AdminCreatePage(c echo.Context) error {
	title := c.FormValue("title")
	slug := c.FormValue("slug")
	content := c.FormValue("content")
	page := models.Page{Title: title, Slug: slug, Content: content, Type: "page"}

	db.DB.Create(&page)
	return c.Redirect(http.StatusFound, "/admin/pages")
}

func AdminPageEditor(c echo.Context) error {
	var page models.Page
	var layouts []models.Layout

	id := c.Param("id")
	if id != "" {
		// Editing existing page
		db.DB.First(&page, id)
	}

	db.DB.Find(&layouts)

	data := map[string]interface{}{
		"Page":    page,
		"Layouts": layouts,
	}

	return renderWithLayout(
		c.Response().Writer,
		"internal/views/admin/admin-layout.html",
		"internal/views/admin/page-editor.html",
		data,
	)
}

// GET /admin/pages/:id/edit
func AdminEditPage(c echo.Context) error {
	id := c.Param("id")
	var page models.Page
	if err := db.DB.First(&page, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Page not found")
	}

	data := map[string]interface{}{
		"Page": page,
	}

	return renderWithLayout(c.Response().Writer, "internal/views/admin/admin-layout.html", "internal/views/admin/edit_page.html", data)
}

// POST /admin/pages/:id/edit
func AdminUpdatePage(c echo.Context) error {
	id := c.Param("id")
	var page models.Page
	if err := db.DB.First(&page, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Page not found")
	}

	page.Title = c.FormValue("title")
	page.Slug = c.FormValue("slug")
	page.Type = c.FormValue("type")
	page.Content = c.FormValue("content")
	layoutIDStr := c.FormValue("layout_id")
	if layoutIDStr != "" {
		if layoutIDUint, err := strconv.ParseUint(layoutIDStr, 10, 64); err == nil {
			page.LayoutID = uint(layoutIDUint)
		} else {
			return c.String(http.StatusBadRequest, "Invalid layout_id")
		}
	}

	if err := db.DB.Save(&page).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to update page")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/pages")
}
