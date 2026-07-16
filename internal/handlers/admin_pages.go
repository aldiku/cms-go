package handlers

import (
	"cms-go/internal/db"
	"cms-go/internal/generator"
	"cms-go/internal/models"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

const adminPagesPerPage = 10

func AdminPages(c echo.Context) error {
	query := strings.TrimSpace(c.QueryParam("q"))

	pageNum, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil || pageNum < 1 {
		pageNum = 1
	}

	like := "%" + query + "%"

	var total int64
	countQuery := db.DB.Model(&models.Page{})
	if query != "" {
		countQuery = countQuery.Where("title ILIKE ? OR slug ILIKE ?", like, like)
	}
	countQuery.Count(&total)

	totalPages := int((total + adminPagesPerPage - 1) / adminPagesPerPage)
	if totalPages < 1 {
		totalPages = 1
	}
	if pageNum > totalPages {
		pageNum = totalPages
	}

	findQuery := db.DB.Model(&models.Page{})
	if query != "" {
		findQuery = findQuery.Where("title ILIKE ? OR slug ILIKE ?", like, like)
	}

	var pages []models.Page
	findQuery.Order("id desc").
		Limit(adminPagesPerPage).
		Offset((pageNum - 1) * adminPagesPerPage).
		Find(&pages)

	data := map[string]interface{}{
		"Pages":       pages,
		"Query":       query,
		"CurrentPage": pageNum,
		"TotalPages":  totalPages,
		"HasPrev":     pageNum > 1,
		"HasNext":     pageNum < totalPages,
		"PrevPage":    pageNum - 1,
		"NextPage":    pageNum + 1,
	}

	return renderWithLayout(c.Response().Writer, "internal/views/admin/admin-layout.html", "internal/views/admin/pages.html", data)
}

func AdminCreatePage(c echo.Context) error {
	title := c.FormValue("title")
	slug := c.FormValue("slug")
	content := c.FormValue("content")
	page := models.Page{Title: title, Slug: slug, Content: content, Type: "page"}

	db.DB.Create(&page)
	if err := generator.GenerateTemplatesFromDB(); err != nil {
		fmt.Println("template generation error:", err)
	}
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

	if err := generator.GenerateTemplatesFromDB(); err != nil {
		fmt.Println("template generation error:", err)
	}

	return c.Redirect(http.StatusSeeOther, "/admin/pages")
}
