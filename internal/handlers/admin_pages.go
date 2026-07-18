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

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/pages.html", data)
}

// bindPageFromForm reads the shared create/edit form fields (meta + SEO)
// from the request into page. Type and layout_id are handled separately by
// each caller since AdminCreatePage hardcodes Type and AdminUpdatePage
// validates layout_id.
func bindPageFromForm(c echo.Context, page *models.Page) {
	page.Title = c.FormValue("title")
	page.Slug = c.FormValue("slug")
	page.Content = c.FormValue("content")

	page.MetaTitle = c.FormValue("meta_title")
	page.MetaDescription = c.FormValue("meta_description")
	page.CanonicalURL = c.FormValue("canonical_url")
	page.FocusKeyword = c.FormValue("focus_keyword")
	page.MetaRobotsNoindex = c.FormValue("meta_robots_noindex") == "on"
	page.MetaRobotsNofollow = c.FormValue("meta_robots_nofollow") == "on"
	page.OGTitle = c.FormValue("og_title")
	page.OGDescription = c.FormValue("og_description")
	page.OGImage = c.FormValue("og_image")
	page.TwitterCard = c.FormValue("twitter_card")
	page.TwitterTitle = c.FormValue("twitter_title")
	page.TwitterDescription = c.FormValue("twitter_description")
	page.TwitterImage = c.FormValue("twitter_image")
}

func AdminCreatePage(c echo.Context) error {
	page := models.Page{Type: "page"}
	bindPageFromForm(c, &page)

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
		c,
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

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/edit_page.html", data)
}

// POST /admin/pages/:id/edit
func AdminUpdatePage(c echo.Context) error {
	id := c.Param("id")
	var page models.Page
	if err := db.DB.First(&page, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Page not found")
	}

	bindPageFromForm(c, &page)
	page.Type = c.FormValue("type")
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
