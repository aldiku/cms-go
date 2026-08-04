package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// GET /admin/product-categories
func AdminProductCategories(c echo.Context) error {
	var categories []models.ProductCategory
	db.DB.Preload("Image").Order("name asc").Find(&categories)

	counts := make(map[uint]int64, len(categories))
	for _, cat := range categories {
		var count int64
		db.DB.Model(&models.Product{}).Where("product_category_id = ?", cat.ID).Count(&count)
		counts[cat.ID] = count
	}

	data := map[string]interface{}{
		"Categories":    categories,
		"ProductCounts": counts,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/product_categories.html", data)
}

// GET /admin/product-categories/new and /admin/product-categories/:id/edit
func AdminProductCategoryForm(c echo.Context) error {
	var category models.ProductCategory
	if id := c.Param("id"); id != "" {
		if err := db.DB.Preload("Image").First(&category, id).Error; err != nil {
			return c.String(http.StatusNotFound, "Product category not found")
		}
	}

	data := map[string]interface{}{
		"Category": category,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/product_category_form.html", data)
}

func bindProductCategoryFromForm(c echo.Context, category *models.ProductCategory) {
	category.Name = c.FormValue("name")
	category.Description = c.FormValue("description")
	if slug := strings.TrimSpace(c.FormValue("slug")); slug != "" {
		category.Slug = slugify(slug)
	} else {
		category.Slug = slugify(category.Name)
	}

	if imageIDStr := c.FormValue("image_id"); imageIDStr != "" {
		if imageID, err := strconv.ParseUint(imageIDStr, 10, 64); err == nil {
			category.ImageID = uint(imageID)
		}
	} else {
		category.ImageID = 0
	}
}

// POST /admin/product-categories/new
func AdminCreateProductCategory(c echo.Context) error {
	var category models.ProductCategory
	bindProductCategoryFromForm(c, &category)
	if category.Name == "" {
		return c.String(http.StatusBadRequest, "Category name is required")
	}
	if err := db.DB.Create(&category).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create category (name/slug must be unique)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/product-categories")
}

// POST /admin/product-categories/:id/edit
func AdminUpdateProductCategory(c echo.Context) error {
	var category models.ProductCategory
	if err := db.DB.First(&category, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Product category not found")
	}
	bindProductCategoryFromForm(c, &category)
	if err := db.DB.Save(&category).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update category (name/slug must be unique)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/product-categories")
}

// POST /admin/product-categories/:id/delete
func AdminDeleteProductCategory(c echo.Context) error {
	var category models.ProductCategory
	if err := db.DB.First(&category, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Product category not found")
	}

	var productCount int64
	db.DB.Model(&models.Product{}).Where("product_category_id = ?", category.ID).Count(&productCount)
	if productCount > 0 {
		return c.String(http.StatusBadRequest, "Cannot delete a category that still has products")
	}

	db.DB.Delete(&category)
	return c.Redirect(http.StatusSeeOther, "/admin/product-categories")
}
