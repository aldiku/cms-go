package handlers

import (
	"net/http"
	"regexp"
	"strings"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// slugify lowercases s and collapses runs of non-alphanumeric characters
// into single hyphens, trimming leading/trailing hyphens.
func slugify(s string) string {
	slug := slugNonAlnum.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	return strings.Trim(slug, "-")
}

// --- Categories ---------------------------------------------------------

// GET /admin/categories
func AdminCategories(c echo.Context) error {
	var categories []models.Category
	db.DB.Order("name asc").Find(&categories)

	counts := make(map[uint]int64, len(categories))
	for _, cat := range categories {
		var count int64
		db.DB.Table("page_categories").Where("category_id = ?", cat.ID).Count(&count)
		counts[cat.ID] = count
	}

	data := map[string]interface{}{
		"Categories": categories,
		"PageCounts": counts,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/categories.html", data)
}

// GET /admin/categories/new and /admin/categories/:id/edit
func AdminCategoryForm(c echo.Context) error {
	var category models.Category
	if id := c.Param("id"); id != "" {
		if err := db.DB.First(&category, id).Error; err != nil {
			return c.String(http.StatusNotFound, "Category not found")
		}
	}

	data := map[string]interface{}{
		"Category": category,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/category_form.html", data)
}

func bindCategoryFromForm(c echo.Context, category *models.Category) {
	category.Name = c.FormValue("name")
	category.Description = c.FormValue("description")
	if slug := strings.TrimSpace(c.FormValue("slug")); slug != "" {
		category.Slug = slugify(slug)
	} else {
		category.Slug = slugify(category.Name)
	}
}

// POST /admin/categories/new
func AdminCreateCategory(c echo.Context) error {
	var category models.Category
	bindCategoryFromForm(c, &category)
	if category.Name == "" {
		return c.String(http.StatusBadRequest, "Category name is required")
	}
	if err := db.DB.Create(&category).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create category (name/slug must be unique)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/categories")
}

// POST /admin/categories/:id/edit
func AdminUpdateCategory(c echo.Context) error {
	var category models.Category
	if err := db.DB.First(&category, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Category not found")
	}
	bindCategoryFromForm(c, &category)
	if err := db.DB.Save(&category).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update category (name/slug must be unique)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/categories")
}

// POST /admin/categories/:id/delete
func AdminDeleteCategory(c echo.Context) error {
	var category models.Category
	if err := db.DB.First(&category, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Category not found")
	}
	db.DB.Exec("DELETE FROM page_categories WHERE category_id = ?", category.ID)
	db.DB.Delete(&category)
	return c.Redirect(http.StatusSeeOther, "/admin/categories")
}

// --- Tags ----------------------------------------------------------------

// GET /admin/tags
func AdminTags(c echo.Context) error {
	var tags []models.Tag
	db.DB.Order("name asc").Find(&tags)

	counts := make(map[uint]int64, len(tags))
	for _, tag := range tags {
		var count int64
		db.DB.Table("page_tags").Where("tag_id = ?", tag.ID).Count(&count)
		counts[tag.ID] = count
	}

	data := map[string]interface{}{
		"Tags":       tags,
		"PageCounts": counts,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/tags.html", data)
}

// GET /admin/tags/new and /admin/tags/:id/edit
func AdminTagForm(c echo.Context) error {
	var tag models.Tag
	if id := c.Param("id"); id != "" {
		if err := db.DB.First(&tag, id).Error; err != nil {
			return c.String(http.StatusNotFound, "Tag not found")
		}
	}

	data := map[string]interface{}{
		"Tag": tag,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/tag_form.html", data)
}

func bindTagFromForm(c echo.Context, tag *models.Tag) {
	tag.Name = c.FormValue("name")
	if slug := strings.TrimSpace(c.FormValue("slug")); slug != "" {
		tag.Slug = slugify(slug)
	} else {
		tag.Slug = slugify(tag.Name)
	}
}

// POST /admin/tags/new
func AdminCreateTag(c echo.Context) error {
	var tag models.Tag
	bindTagFromForm(c, &tag)
	if tag.Name == "" {
		return c.String(http.StatusBadRequest, "Tag name is required")
	}
	if err := db.DB.Create(&tag).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create tag (name/slug must be unique)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/tags")
}

// POST /admin/tags/:id/edit
func AdminUpdateTag(c echo.Context) error {
	var tag models.Tag
	if err := db.DB.First(&tag, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Tag not found")
	}
	bindTagFromForm(c, &tag)
	if err := db.DB.Save(&tag).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update tag (name/slug must be unique)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/tags")
}

// POST /admin/tags/:id/delete
func AdminDeleteTag(c echo.Context) error {
	var tag models.Tag
	if err := db.DB.First(&tag, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Tag not found")
	}
	db.DB.Exec("DELETE FROM page_tags WHERE tag_id = ?", tag.ID)
	db.DB.Delete(&tag)
	return c.Redirect(http.StatusSeeOther, "/admin/tags")
}
