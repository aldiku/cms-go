package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// variantFormView is the display shape for one variant create/edit form —
// built here so the template can stay a simple range with no map-literal
// template-function tricks, matching e.g. sessionView in auth_account.go.
type variantFormView struct {
	Action        string
	Variant       models.ProductVariant
	IsNew         bool
	Tiers         []models.ProductVariantTier
	PricingAction string // POST target for the "grant a custom price" form
	Overrides     []overrideDisplay
}

// overrideDisplay is one row of a variant's Custom Pricing panel — the
// target user's name/email resolved for display alongside the override.
type overrideDisplay struct {
	ID           uint
	TargetUserID uint
	TargetName   string
	TargetEmail  string
	Price        int64
	DeleteAction string
}

// GET /admin/products/:id/variants
func AdminProductVariants(c echo.Context) error {
	var product models.Product
	if err := db.DB.First(&product, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Product not found")
	}
	base := "/admin/products/" + strconv.FormatUint(uint64(product.ID), 10) + "/variants"

	var variants []models.ProductVariant
	db.DB.Where("product_id = ?", product.ID).Order("list_order asc, id asc").Find(&variants)

	forms := make([]variantFormView, 0, len(variants))
	for _, v := range variants {
		var tiers []models.ProductVariantTier
		db.DB.Where("variant_id = ?", v.ID).Order("id asc").Find(&tiers)

		vBase := base + "/" + strconv.FormatUint(uint64(v.ID), 10)

		var overrideRows []models.PriceOverride
		db.DB.Where("variant_id = ?", v.ID).Order("id asc").Find(&overrideRows)
		overrides := make([]overrideDisplay, 0, len(overrideRows))
		for _, o := range overrideRows {
			var target models.User
			db.DB.First(&target, o.TargetUserID)
			overrides = append(overrides, overrideDisplay{
				ID: o.ID, TargetUserID: o.TargetUserID,
				TargetName: target.FullName(), TargetEmail: target.Email,
				Price:        o.Price,
				DeleteAction: vBase + "/pricing/" + strconv.FormatUint(uint64(o.ID), 10) + "/delete",
			})
		}

		forms = append(forms, variantFormView{
			Action:        vBase + "/edit",
			Variant:       v,
			Tiers:         tiers,
			PricingAction: vBase + "/pricing/new",
			Overrides:     overrides,
		})
	}

	newForm := variantFormView{
		Action: base + "/new",
		IsNew:  true,
		Variant: models.ProductVariant{
			PricingMode: "fixed",
			Status:      1,
		},
	}

	data := map[string]interface{}{
		"Product": product,
		"NewForm": newForm,
		"Forms":   forms,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/product_variants.html", data)
}

func bindVariantFromForm(c echo.Context, variant *models.ProductVariant) {
	variant.Name = c.FormValue("name")
	variant.PricingMode = c.FormValue("pricing_mode")
	if variant.PricingMode != "tiering" {
		variant.PricingMode = "fixed"
	}
	if hpp, err := strconv.ParseInt(c.FormValue("hpp"), 10, 64); err == nil {
		variant.HPP = hpp
	}
	if price, err := strconv.ParseInt(c.FormValue("price"), 10, 64); err == nil {
		variant.Price = price
	}
	if c.FormValue("status") == "on" {
		variant.Status = 1
	} else {
		variant.Status = 0
	}
}

// replaceVariantTiers deletes and recreates every ProductVariantTier row
// for variantID from the submitted tier_label[]/tier_price[]/
// tier_is_custom[] parallel arrays — simplest correct approach for the
// handful of rows a variant's tiering ever has.
func replaceVariantTiers(c echo.Context, variantID uint) {
	db.DB.Delete(&models.ProductVariantTier{}, "variant_id = ?", variantID)

	form, _ := c.FormParams()
	labels := form["tier_label[]"]
	prices := form["tier_price[]"]
	isCustoms := form["tier_is_custom[]"]

	for i, label := range labels {
		if label == "" {
			continue
		}
		var price int64
		if i < len(prices) {
			price, _ = strconv.ParseInt(prices[i], 10, 64)
		}
		isCustom := i < len(isCustoms) && isCustoms[i] == "true"

		tier := models.ProductVariantTier{
			VariantID: variantID,
			Label:     label,
			Price:     price,
			IsCustom:  isCustom,
		}
		db.DB.Create(&tier)
	}
}

// POST /admin/products/:id/variants/new
func AdminCreateProductVariant(c echo.Context) error {
	var product models.Product
	if err := db.DB.First(&product, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Product not found")
	}

	var variant models.ProductVariant
	bindVariantFromForm(c, &variant)
	if variant.Name == "" {
		return c.String(http.StatusBadRequest, "Variant name is required")
	}
	variant.ProductID = product.ID

	var maxOrder uint32
	db.DB.Model(&models.ProductVariant{}).Where("product_id = ?", product.ID).
		Select("COALESCE(MAX(list_order), 0)").Scan(&maxOrder)
	variant.ListOrder = maxOrder + 1

	if err := db.DB.Create(&variant).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create variant")
	}
	if variant.PricingMode == "tiering" {
		replaceVariantTiers(c, variant.ID)
	}

	return c.Redirect(http.StatusSeeOther, "/admin/products/"+strconv.FormatUint(uint64(product.ID), 10)+"/variants")
}

// POST /admin/products/:id/variants/:variant_id/edit
func AdminUpdateProductVariant(c echo.Context) error {
	var variant models.ProductVariant
	if err := db.DB.First(&variant, c.Param("variant_id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Variant not found")
	}
	bindVariantFromForm(c, &variant)
	if err := db.DB.Save(&variant).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update variant")
	}

	if variant.PricingMode == "tiering" {
		replaceVariantTiers(c, variant.ID)
	} else {
		db.DB.Delete(&models.ProductVariantTier{}, "variant_id = ?", variant.ID)
	}

	return c.Redirect(http.StatusSeeOther, "/admin/products/"+c.Param("id")+"/variants")
}

// POST /admin/products/:id/variants/:variant_id/delete
func AdminDeleteProductVariant(c echo.Context) error {
	var variant models.ProductVariant
	if err := db.DB.First(&variant, c.Param("variant_id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Variant not found")
	}
	db.DB.Delete(&models.ProductVariantTier{}, "variant_id = ?", variant.ID)
	db.DB.Delete(&variant)
	return c.Redirect(http.StatusSeeOther, "/admin/products/"+c.Param("id")+"/variants")
}

type variantSearchJSON struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	ProductName string `json:"productName"`
	Price       int64  `json:"price"`
}

// GET /admin/products/variants/json?q=... — lightweight search across every product
// variant in every category/product, matching by variant name or parent
// product name. Used by pickers that need to target an arbitrary variant
// (e.g. the Custom Pricing "add for this user" flow, admin_price_overrides.go),
// unlike the Variant page's own picker which already has a fixed variant.
func AdminVariantsJSON(c echo.Context) error {
	query := strings.TrimSpace(c.QueryParam("q"))

	q := db.DB.Model(&models.ProductVariant{})
	if query != "" {
		like := "%" + query + "%"
		var matchingProductIDs []uint
		db.DB.Model(&models.Product{}).Where("name ILIKE ?", like).Pluck("id", &matchingProductIDs)
		if len(matchingProductIDs) > 0 {
			q = q.Where("name ILIKE ? OR product_id IN ?", like, matchingProductIDs)
		} else {
			q = q.Where("name ILIKE ?", like)
		}
	}

	var variants []models.ProductVariant
	q.Order("name asc").Limit(20).Find(&variants)

	productIDs := make([]uint, 0, len(variants))
	seen := map[uint]bool{}
	for _, v := range variants {
		if !seen[v.ProductID] {
			seen[v.ProductID] = true
			productIDs = append(productIDs, v.ProductID)
		}
	}
	var products []models.Product
	if len(productIDs) > 0 {
		db.DB.Where("id IN ?", productIDs).Find(&products)
	}
	productNames := make(map[uint]string, len(products))
	for _, p := range products {
		productNames[p.ID] = p.Name
	}

	out := make([]variantSearchJSON, 0, len(variants))
	for _, v := range variants {
		out = append(out, variantSearchJSON{
			ID: v.ID, Name: v.Name, ProductName: productNames[v.ProductID], Price: v.Price,
		})
	}
	return c.JSON(http.StatusOK, out)
}
