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
	quantities := form["tier_quantity[]"]
	prices := form["tier_price[]"]
	isCustoms := form["tier_is_custom[]"]

	for i, label := range labels {
		if label == "" {
			continue
		}
		var price, quantity int64
		if i < len(prices) {
			price, _ = strconv.ParseInt(prices[i], 10, 64)
		}
		if i < len(quantities) {
			quantity, _ = strconv.ParseInt(quantities[i], 10, 64)
		}
		isCustom := i < len(isCustoms) && isCustoms[i] == "true"

		tier := models.ProductVariantTier{
			VariantID: variantID,
			Label:     label,
			Price:     price,
			Quantity:  quantity,
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
	VariantID    uint   `json:"variant_id"` // 0 when HasVariant is false
	ProductID    uint   `json:"product_id"`
	Name         string `json:"name"`
	ProductName  string `json:"product_name"`
	Price        int64  `json:"price"`         // effective: target_user_id's override if one exists, else DefaultPrice
	DefaultPrice int64  `json:"default_price"` // catalog price (variant's own, or the product's base if no variant) — ignores any override
	PricingMode  string `json:"pricing_mode"`  // "fixed" | "tiering" | "" (no variant yet)
	IsCustom     bool   `json:"is_custom"`     // true when Price is target_user_id's existing override
	HasVariant   bool   `json:"has_variant"`   // false = picking this auto-creates a default variant
}

// variantDisplayPrice resolves a variant's own catalog price: the fixed
// price, or (tiering has no single "the" price — its Price field is
// unused) the cheapest non-custom tier.
func variantDisplayPrice(v models.ProductVariant) int64 {
	if v.PricingMode == "fixed" {
		return v.Price
	}
	var tiers []models.ProductVariantTier
	db.DB.Where("variant_id = ?", v.ID).Order("id asc").Find(&tiers)
	cheapest := int64(-1)
	for _, t := range tiers {
		if t.IsCustom {
			continue
		}
		if cheapest == -1 || t.Price < cheapest {
			cheapest = t.Price
		}
	}
	if cheapest == -1 {
		return 0
	}
	return cheapest
}

// GET /admin/products/variants/json?q=...&target_user_id=... — lightweight
// search used by the Custom Pricing "add for this user" flow
// (admin_price_overrides.go, custom_pricing_user.html) to pick what to
// grant a price override on.
//
// Matches the pricing hierarchy note.md documents for a product: its own
// base Price, overridden by a ProductVariant's price when one exists,
// overridden again by a PriceOverride for the specific target user. So
// products with NO variant configured yet are included too (priced at
// their base Price, HasVariant: false) rather than being invisible to this
// search — previously a product with no variant simply couldn't be found
// here, so it could never be granted a custom price at all. Picking one
// auto-provisions a default fixed-price variant seeded from the product's
// base price (see AdminSetPriceOverrideForUser), since PriceOverride is
// always anchored to a real variant. When target_user_id is given, Price/
// IsCustom reflect that user's existing override if one already exists,
// so the admin sees what they're currently paying, not just the catalog
// default.
func AdminVariantsJSON(c echo.Context) error {
	query := strings.TrimSpace(c.QueryParam("q"))
	targetUserID, _ := strconv.ParseUint(c.QueryParam("target_user_id"), 10, 64)

	var matchingProductIDs []uint
	vq := db.DB.Model(&models.ProductVariant{})
	if query != "" {
		like := "%" + query + "%"
		db.DB.Model(&models.Product{}).Where("name ILIKE ?", like).Pluck("id", &matchingProductIDs)
		if len(matchingProductIDs) > 0 {
			vq = vq.Where("name ILIKE ? OR product_id IN ?", like, matchingProductIDs)
		} else {
			vq = vq.Where("name ILIKE ?", like)
		}
	}

	var variants []models.ProductVariant
	vq.Order("name asc").Limit(20).Find(&variants)

	hasVariant := map[uint]bool{}
	productIDs := append([]uint{}, matchingProductIDs...)
	for _, v := range variants {
		hasVariant[v.ProductID] = true
		productIDs = append(productIDs, v.ProductID)
	}
	var products []models.Product
	if len(productIDs) > 0 {
		db.DB.Where("id IN ?", productIDs).Find(&products)
	}
	productByID := make(map[uint]models.Product, len(products))
	for _, p := range products {
		productByID[p.ID] = p
	}

	out := make([]variantSearchJSON, 0, 20)
	for _, v := range variants {
		defaultPrice := variantDisplayPrice(v)
		price := defaultPrice
		isCustom := false
		if targetUserID > 0 {
			var override models.PriceOverride
			if db.DB.Where("variant_id = ? AND target_user_id = ?", v.ID, targetUserID).First(&override).Error == nil {
				price, isCustom = override.Price, true
			}
		}
		out = append(out, variantSearchJSON{
			VariantID: v.ID, ProductID: v.ProductID, Name: v.Name,
			ProductName: productByID[v.ProductID].Name, Price: price, DefaultPrice: defaultPrice,
			PricingMode: v.PricingMode, IsCustom: isCustom, HasVariant: true,
		})
	}
	for _, id := range matchingProductIDs {
		if hasVariant[id] {
			continue
		}
		p := productByID[id]
		out = append(out, variantSearchJSON{
			ProductID: p.ID, Name: "Base Price", ProductName: p.Name,
			Price: p.Price, DefaultPrice: p.Price, HasVariant: false,
		})
	}

	if len(out) > 20 {
		out = out[:20]
	}
	return c.JSON(http.StatusOK, out)
}
