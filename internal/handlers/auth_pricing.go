// Reseller-facing Custom Pricing self-service (/auth/pricing) — session-only,
// no RBAC gate, same pattern as /auth/profile and /auth/setting
// (auth_account.go). Any logged-in user can visit; the page shows an empty
// state if they have no downline (no User rows with ReferralID = them).
package handlers

import (
	"net/http"
	"strconv"

	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/pricing"

	"github.com/labstack/echo/v4"
)

// pricingRow is one line of the reseller's pricing table for a selected
// client: what the reseller pays ("your price") vs what the client pays,
// and the resulting margin.
type pricingRow struct {
	VariantID   uint
	VariantName string
	ProductName string
	YourPrice   int64
	ClientPrice int64
	Margin      int64
}

// GET /auth/pricing?client=N
func AuthPricing(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}

	var downline []models.User
	db.DB.Where("referral_id = ?", current.ID).Order("firstname asc").Find(&downline)

	data := map[string]interface{}{
		"Downline": downline,
	}

	if clientIDStr := c.QueryParam("client"); clientIDStr != "" {
		var client models.User
		if err := db.DB.Where("id = ? AND referral_id = ?", clientIDStr, current.ID).First(&client).Error; err == nil {
			data["SelectedClient"] = client

			var variants []models.ProductVariant
			db.DB.Where("status = 1").Order("name asc").Find(&variants)

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

			rows := make([]pricingRow, 0, len(variants))
			for _, v := range variants {
				yourPrice, _ := pricing.EffectivePrice(v.ID, current.ID)
				clientPrice, _ := pricing.EffectivePrice(v.ID, client.ID)
				rows = append(rows, pricingRow{
					VariantID:   v.ID,
					VariantName: v.Name,
					ProductName: productNames[v.ProductID],
					YourPrice:   yourPrice,
					ClientPrice: clientPrice,
					Margin:      clientPrice - yourPrice,
				})
			}
			data["Rows"] = rows
		}
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/pricing.html", data)
}

// POST /auth/pricing/set — a reseller granting a custom price to one of
// their own clients. client.ReferralID must equal the current user's ID;
// anyone else is refused, unlike the admin equivalent (AdminSetPriceOverride,
// admin_price_overrides.go) which can target any user.
func AuthSetClientPrice(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}

	clientID, err := strconv.ParseUint(c.FormValue("client_id"), 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid client")
	}
	variantID, err := strconv.ParseUint(c.FormValue("variant_id"), 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid variant")
	}
	price, err := strconv.ParseInt(c.FormValue("price"), 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "A valid price is required")
	}

	var client models.User
	if err := db.DB.Where("id = ? AND referral_id = ?", clientID, current.ID).First(&client).Error; err != nil {
		return c.String(http.StatusForbidden, "You can only set prices for your own clients")
	}

	override := models.PriceOverride{VariantID: uint(variantID), TargetUserID: uint(clientID)}
	if err := db.DB.Where(override).
		Assign(models.PriceOverride{Price: price, SetByUserID: current.ID}).
		FirstOrCreate(&override).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to save price")
	}

	return c.Redirect(http.StatusSeeOther, "/auth/pricing?client="+strconv.FormatUint(clientID, 10))
}
