package handlers

import (
	"cms-go/internal/db"
	"cms-go/internal/models"
	"net/http"

	"github.com/labstack/echo/v4"
)

func AdminCreatePage(c echo.Context) error {
	title := c.FormValue("title")
	slug := c.FormValue("slug")
	content := c.FormValue("content")
	page := models.Page{Title: title, Slug: slug, Content: content, Type: "page"}

	db.DB.Create(&page)
	return c.Redirect(http.StatusFound, "/admin/pages")
}

// GET /admin/pages/:id/edit
func AdminEditPage(c echo.Context) error {
	id := c.Param("id")
	var page models.Page
	if err := db.DB.First(&page, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Page not found")
	}
	return c.Render(http.StatusOK, "edit_page.html", map[string]interface{}{
		"Page": page,
	})
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

	if err := db.DB.Save(&page).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to update page")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/pages")
}
