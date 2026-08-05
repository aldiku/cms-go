package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"cms-go/internal/auth"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// POST /admin/products/:id/variants/:variant_id/pricing/new — grants (or
// updates, if one already exists for this variant+user) a custom price.
// Admin may grant to any user, unlike the reseller-facing equivalent
// (AuthSetClientPrice, internal/handlers/auth_pricing.go) which is
// restricted to the reseller's own downline.
func AdminSetPriceOverride(c echo.Context) error {
	variantID, err := strconv.ParseUint(c.Param("variant_id"), 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid variant")
	}
	targetUserID, err := strconv.ParseUint(c.FormValue("target_user_id"), 10, 64)
	if err != nil || targetUserID == 0 {
		return c.String(http.StatusBadRequest, "A target user is required")
	}
	price, err := strconv.ParseInt(c.FormValue("price"), 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "A valid price is required")
	}

	var target models.User
	if err := db.DB.First(&target, targetUserID).Error; err != nil {
		return c.String(http.StatusBadRequest, "Target user not found")
	}

	setByID := uint(0)
	if current, ok := c.Get(auth.CtxUser).(models.User); ok {
		setByID = current.ID
	}

	override := models.PriceOverride{
		VariantID:    uint(variantID),
		TargetUserID: uint(targetUserID),
	}
	if err := db.DB.Where(override).
		Assign(models.PriceOverride{Price: price, SetByUserID: setByID}).
		FirstOrCreate(&override).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to save custom price")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/products/"+c.Param("id")+"/variants")
}

// POST /admin/products/:id/variants/:variant_id/pricing/:override_id/delete
func AdminDeletePriceOverride(c echo.Context) error {
	db.DB.Delete(&models.PriceOverride{}, "id = ?", c.Param("override_id"))
	return c.Redirect(http.StatusSeeOther, "/admin/products/"+c.Param("id")+"/variants")
}

// customPricingRow is one line of the global Custom Pricing list — every
// related name resolved up front so the template stays a simple range.
type customPricingRow struct {
	ID            uint
	ProductID     uint
	ProductName   string
	VariantID     uint
	VariantName   string
	TargetName    string
	TargetEmail   string
	SetByName     string
	OriginalPrice int64 // the variant's own catalog price, i.e. what Price is overriding
	Price         int64
}

// buildCustomPricingRows resolves the product/variant/user names for a set
// of overrides in a handful of batched queries (no N+1), optionally
// filtering by a lowercase search term across product/variant/target
// name/email. Shared by the global list and the per-user view.
func buildCustomPricingRows(overrides []models.PriceOverride, query string) []customPricingRow {
	variantIDs := make([]uint, 0, len(overrides))
	userIDs := make([]uint, 0, len(overrides)*2)
	seenVariant := map[uint]bool{}
	seenUser := map[uint]bool{}
	for _, o := range overrides {
		if !seenVariant[o.VariantID] {
			seenVariant[o.VariantID] = true
			variantIDs = append(variantIDs, o.VariantID)
		}
		for _, uid := range []uint{o.TargetUserID, o.SetByUserID} {
			if uid != 0 && !seenUser[uid] {
				seenUser[uid] = true
				userIDs = append(userIDs, uid)
			}
		}
	}

	var variants []models.ProductVariant
	if len(variantIDs) > 0 {
		db.DB.Where("id IN ?", variantIDs).Find(&variants)
	}
	productIDs := make([]uint, 0, len(variants))
	seenProduct := map[uint]bool{}
	variantByID := make(map[uint]models.ProductVariant, len(variants))
	for _, v := range variants {
		variantByID[v.ID] = v
		if !seenProduct[v.ProductID] {
			seenProduct[v.ProductID] = true
			productIDs = append(productIDs, v.ProductID)
		}
	}

	var products []models.Product
	if len(productIDs) > 0 {
		db.DB.Where("id IN ?", productIDs).Find(&products)
	}
	productByID := make(map[uint]models.Product, len(products))
	for _, p := range products {
		productByID[p.ID] = p
	}

	var users []models.User
	if len(userIDs) > 0 {
		db.DB.Where("id IN ?", userIDs).Find(&users)
	}
	userByID := make(map[uint]models.User, len(users))
	for _, u := range users {
		userByID[u.ID] = u
	}

	rows := make([]customPricingRow, 0, len(overrides))
	for _, o := range overrides {
		variant := variantByID[o.VariantID]
		product := productByID[variant.ProductID]
		target := userByID[o.TargetUserID]
		setBy := userByID[o.SetByUserID]

		if query != "" {
			haystack := strings.ToLower(product.Name + " " + variant.Name + " " + target.FullName() + " " + target.Email)
			if !strings.Contains(haystack, query) {
				continue
			}
		}

		rows = append(rows, customPricingRow{
			ID: o.ID, ProductID: product.ID, ProductName: product.Name,
			VariantID: variant.ID, VariantName: variant.Name,
			TargetName: target.FullName(), TargetEmail: target.Email,
			SetByName: setBy.FullName(), OriginalPrice: variantDisplayPrice(variant), Price: o.Price,
		})
	}
	return rows
}

// GET /admin/custom-pricing — every price override across every variant
// and user, searchable by product/variant/target name/email. This is a
// global browse/audit view; use "+ Add" (-> AdminCustomPricingForUser) to
// manage one user's prices across variants.
func AdminCustomPricingList(c echo.Context) error {
	query := strings.ToLower(strings.TrimSpace(c.QueryParam("q")))

	var overrides []models.PriceOverride
	db.DB.Order("updated_at desc").Find(&overrides)

	data := map[string]interface{}{
		"Rows":  buildCustomPricingRows(overrides, query),
		"Query": c.QueryParam("q"),
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/custom_pricing.html", data)
}

// GET /admin/custom-pricing/user/:user_id — the "add" wizard's second
// step: search picks a user (AdminUsersJSON), this page shows everything
// that user already has and lets the admin edit any of it or grant a
// price for another variant (AdminVariantsJSON picks the variant).
func AdminCustomPricingForUser(c echo.Context) error {
	var target models.User
	if err := db.DB.First(&target, c.Param("user_id")).Error; err != nil {
		return c.String(http.StatusNotFound, "User not found")
	}

	var overrides []models.PriceOverride
	db.DB.Where("target_user_id = ?", target.ID).Order("updated_at desc").Find(&overrides)

	data := map[string]interface{}{
		"Target":     target,
		"Rows":       buildCustomPricingRows(overrides, ""),
		"SetAction":  "/admin/custom-pricing/user/" + c.Param("user_id") + "/set",
		"PageAction": "/admin/custom-pricing/user/" + c.Param("user_id"),
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/custom_pricing_user.html", data)
}

// POST /admin/custom-pricing/user/:user_id/set — grants (or updates) a
// price for an admin-chosen variant, for the user fixed by the URL. Same
// upsert as AdminSetPriceOverride, just with variant_id coming from the
// form instead of the path (this flow starts from the user, not a variant).
//
// variant_id is 0 when the picker (AdminVariantsJSON) matched a product
// that has no variant configured yet — PriceOverride is always anchored to
// a real ProductVariant, so in that case a default fixed-price variant is
// auto-provisioned here, seeded from the product's base Price, and the
// override is granted against that instead. product_id is only consulted
// in that case.
func AdminSetPriceOverrideForUser(c echo.Context) error {
	targetUserID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid user")
	}
	price, err := strconv.ParseInt(c.FormValue("price"), 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "A valid price is required")
	}

	variantID, _ := strconv.ParseUint(c.FormValue("variant_id"), 10, 64)
	if variantID == 0 {
		productID, err := strconv.ParseUint(c.FormValue("product_id"), 10, 64)
		if err != nil || productID == 0 {
			return c.String(http.StatusBadRequest, "A product or variant is required")
		}
		var product models.Product
		if err := db.DB.First(&product, productID).Error; err != nil {
			return c.String(http.StatusBadRequest, "Product not found")
		}
		variant := models.ProductVariant{
			ProductID: product.ID, Name: "Base Price",
			PricingMode: "fixed", Price: product.Price, Status: 1,
		}
		if err := db.DB.Create(&variant).Error; err != nil {
			return c.String(http.StatusInternalServerError, "Failed to create default variant")
		}
		variantID = uint64(variant.ID)
	} else if db.DB.First(&models.ProductVariant{}, variantID).Error != nil {
		return c.String(http.StatusBadRequest, "Variant not found")
	}

	setByID := uint(0)
	if current, ok := c.Get(auth.CtxUser).(models.User); ok {
		setByID = current.ID
	}

	override := models.PriceOverride{
		VariantID:    uint(variantID),
		TargetUserID: uint(targetUserID),
	}
	if err := db.DB.Where(override).
		Assign(models.PriceOverride{Price: price, SetByUserID: setByID}).
		FirstOrCreate(&override).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to save custom price")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/custom-pricing/user/"+c.Param("user_id"))
}

// POST /admin/custom-pricing/:override_id/delete — redirects back to
// wherever the delete was requested from (the global list by default, or
// a specific user's page if redirect_to says so) rather than always
// bouncing to the global list.
func AdminDeleteCustomPricing(c echo.Context) error {
	db.DB.Delete(&models.PriceOverride{}, "id = ?", c.Param("override_id"))
	if redirectTo := c.FormValue("redirect_to"); strings.HasPrefix(redirectTo, "/admin/custom-pricing/user/") {
		return c.Redirect(http.StatusSeeOther, redirectTo)
	}
	return c.Redirect(http.StatusSeeOther, "/admin/custom-pricing")
}
