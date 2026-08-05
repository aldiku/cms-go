package handlers

import (
	"net/http"
	"strconv"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// ProductNode is one entry in the tree rendered for a product category: a
// product plus its children, in list_order.
type ProductNode struct {
	Product  models.Product
	Children []ProductNode
}

func buildProductTree(products []models.Product) []ProductNode {
	childrenOf := map[uint][]models.Product{}
	for _, p := range products {
		childrenOf[p.ParentID] = append(childrenOf[p.ParentID], p)
	}
	var build func(parentID uint) []ProductNode
	build = func(parentID uint) []ProductNode {
		kids := childrenOf[parentID]
		nodes := make([]ProductNode, 0, len(kids))
		for _, p := range kids {
			nodes = append(nodes, ProductNode{Product: p, Children: build(p.ID)})
		}
		return nodes
	}
	return build(0)
}

// resolveProductCategory loads the requested category (by id, via
// ?category=) or falls back to the first category alphabetically.
func resolveProductCategory(c echo.Context) (models.ProductCategory, error) {
	var category models.ProductCategory
	if idStr := c.QueryParam("category"); idStr != "" {
		if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
			if err := db.DB.First(&category, id).Error; err == nil {
				return category, nil
			}
		}
	}
	if err := db.DB.Order("name asc").First(&category).Error; err != nil {
		return category, err
	}
	return category, nil
}

// GET /admin/products?category=N
func AdminProducts(c echo.Context) error {
	category, err := resolveProductCategory(c)
	if err != nil {
		return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/products.html", map[string]interface{}{
			"NoCategories": true,
		})
	}

	var categories []models.ProductCategory
	db.DB.Order("name asc").Find(&categories)

	var products []models.Product
	db.DB.Preload("Thumbnail").Where("product_category_id = ?", category.ID).
		Order("list_order asc, id asc").Find(&products)

	data := map[string]interface{}{
		"Categories":       categories,
		"SelectedCategory": category,
		"ProductTree":      buildProductTree(products),
		"HasProducts":      len(products) > 0,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/products.html", data)
}

func bindProductFromForm(c echo.Context, product *models.Product) {
	product.Code = c.FormValue("code")
	product.Name = c.FormValue("name")
	product.Description = c.FormValue("description")

	if imageIDStr := c.FormValue("thumbnail_id"); imageIDStr != "" {
		if imageID, err := strconv.ParseUint(imageIDStr, 10, 64); err == nil {
			product.ThumbnailID = uint(imageID)
		}
	} else {
		product.ThumbnailID = 0
	}

	if hpp, err := strconv.ParseInt(c.FormValue("hpp"), 10, 64); err == nil {
		product.HPP = hpp
	}
	if price, err := strconv.ParseInt(c.FormValue("price"), 10, 64); err == nil {
		product.Price = price
	}
	if c.FormValue("status") == "on" {
		product.Status = 1
	} else {
		product.Status = 0
	}

	if minQuota, err := strconv.ParseInt(c.FormValue("min_quota"), 10, 64); err == nil {
		product.MinQuota = minQuota
	}
	product.IsCampaignable = c.FormValue("is_campaignable") == "on"
}

// POST /admin/products/new — always creates at the root of the submitted
// category; use the tree's drag-and-drop to nest or reorder afterward.
func AdminCreateProduct(c echo.Context) error {
	categoryID, err := strconv.ParseUint(c.FormValue("category_id"), 10, 64)
	if err != nil || categoryID == 0 {
		return c.String(http.StatusBadRequest, "Missing product category")
	}
	var category models.ProductCategory
	if err := db.DB.First(&category, categoryID).Error; err != nil {
		return c.String(http.StatusNotFound, "Product category not found")
	}

	var product models.Product
	bindProductFromForm(c, &product)
	if product.Code == "" || product.Name == "" {
		return c.String(http.StatusBadRequest, "Product code and name are required")
	}
	product.ProductCategoryID = category.ID
	product.ParentID = 0

	var maxOrder uint32
	db.DB.Model(&models.Product{}).Where("product_category_id = ?", category.ID).
		Select("COALESCE(MAX(list_order), 0)").Scan(&maxOrder)
	product.ListOrder = maxOrder + 1

	if err := db.DB.Create(&product).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create product (code must be unique)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/products?category="+strconv.FormatUint(uint64(category.ID), 10))
}

// POST /admin/products/:id/edit — content-only update; structure
// (parent/order) is changed exclusively via drag-and-drop through
// AdminReorderProducts.
func AdminUpdateProduct(c echo.Context) error {
	var product models.Product
	if err := db.DB.First(&product, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Product not found")
	}
	bindProductFromForm(c, &product)
	if err := db.DB.Save(&product).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update product (code must be unique)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/products?category="+strconv.FormatUint(uint64(product.ProductCategoryID), 10))
}

// POST /admin/products/:id/delete
func AdminDeleteProduct(c echo.Context) error {
	var product models.Product
	if err := db.DB.First(&product, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Product not found")
	}

	var childCount int64
	db.DB.Model(&models.Product{}).Where("parent_id = ?", product.ID).Count(&childCount)
	if childCount > 0 {
		return c.String(http.StatusBadRequest, "Cannot delete a product that still has children")
	}

	var variantCount int64
	db.DB.Model(&models.ProductVariant{}).Where("product_id = ?", product.ID).Count(&variantCount)
	if variantCount > 0 {
		return c.String(http.StatusBadRequest, "Cannot delete a product that still has variants")
	}

	categoryID := product.ProductCategoryID
	db.DB.Delete(&product)
	return c.Redirect(http.StatusSeeOther, "/admin/products?category="+strconv.FormatUint(uint64(categoryID), 10))
}

type productReorderItem struct {
	ID       uint   `json:"id"`
	ParentID uint   `json:"parent_id"`
	Order    uint32 `json:"order"`
}

type productReorderRequest struct {
	CategoryID uint                 `json:"category_id"`
	Items      []productReorderItem `json:"items"`
}

// POST /admin/products/reorder — persists the tree's structure (parent +
// order) after a drag-and-drop move. All items must belong to the
// submitted category.
func AdminReorderProducts(c echo.Context) error {
	var req productReorderRequest
	if err := c.Bind(&req); err != nil || req.CategoryID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ids := make([]uint, 0, len(req.Items))
	parentByID := map[uint]uint{}
	for _, item := range req.Items {
		if item.ParentID == item.ID {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "a product cannot be its own parent"})
		}
		ids = append(ids, item.ID)
		parentByID[item.ID] = item.ParentID
	}
	if len(ids) == 0 {
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	}

	// Every non-root parent must itself be one of the items being
	// reordered (i.e. actually belong to this category) — guards against
	// cross-category reparenting from a tampered request.
	inSet := make(map[uint]bool, len(ids))
	for _, id := range ids {
		inSet[id] = true
	}
	for _, parentID := range parentByID {
		if parentID != 0 && !inSet[parentID] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid parent"})
		}
	}

	var count int64
	db.DB.Model(&models.Product{}).Where("id IN ? AND product_category_id = ?", ids, req.CategoryID).Count(&count)
	if int(count) != len(ids) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "items do not match product category"})
	}

	tx := db.DB.Begin()
	for _, item := range req.Items {
		if err := tx.Model(&models.Product{}).Where("id = ?", item.ID).
			Updates(map[string]interface{}{"parent_id": item.ParentID, "list_order": item.Order}).Error; err != nil {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save order"})
		}
	}
	tx.Commit()

	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
