// Self-service Channel management (/auth/channels) — session-only, no RBAC
// gate, same tier as /auth/pricing and /auth/profile. An owner manages only
// the channels they own; creating new channels stays admin-only (see
// admin_channels.go and the plan's scope decisions).
package handlers

import (
	"net/http"
	"strconv"

	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/pricing"

	"github.com/labstack/echo/v4"
)

// GET /auth/channels
func AuthChannels(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}

	var channels []models.Channel
	db.DB.Where("owner_user_id = ?", current.ID).Order("created_at desc").Find(&channels)

	data := map[string]interface{}{
		"Cards": loadChannelCards(channels),
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/my_channels.html", data)
}

// smsTopupProduct returns the smstopup Product and its first active
// variant (note.md §9.4 implies a single price point, not a picker).
func smsTopupProduct() (models.Product, models.ProductVariant, bool) {
	var product models.Product
	if err := db.DB.Where("code = ?", "smstopup").First(&product).Error; err != nil {
		return product, models.ProductVariant{}, false
	}
	var variant models.ProductVariant
	if err := db.DB.Where("product_id = ? AND status = 1", product.ID).Order("list_order asc, id asc").First(&variant).Error; err != nil {
		return product, variant, false
	}
	return product, variant, true
}

// wabaTopupOption is one selectable WABA topup product for the self-service
// picker, with its variants (and each variant's tiers, if tiering).
type wabaTopupOption struct {
	Product  models.Product
	Variants []wabaVariantOption
}

type wabaVariantOption struct {
	Variant models.ProductVariant
	Tiers   []models.ProductVariantTier
}

// wabaTopupOptions lists every non-campaignable product other than
// smstopup (note.md §9.6: that's exactly the 4 WABA topup products) with
// their pricing, so WABA topup choices come straight from the catalog
// instead of a hardcoded preset list like SMS.
func wabaTopupOptions() []wabaTopupOption {
	var products []models.Product
	db.DB.Where("is_campaignable = ? AND code <> ?", false, "smstopup").Order("name asc").Find(&products)

	options := make([]wabaTopupOption, 0, len(products))
	for _, p := range products {
		var variants []models.ProductVariant
		db.DB.Where("product_id = ? AND status = 1", p.ID).Order("list_order asc, id asc").Find(&variants)

		variantOptions := make([]wabaVariantOption, 0, len(variants))
		for _, v := range variants {
			var tiers []models.ProductVariantTier
			if v.PricingMode == "tiering" {
				db.DB.Where("variant_id = ?", v.ID).Order("id asc").Find(&tiers)
			}
			variantOptions = append(variantOptions, wabaVariantOption{Variant: v, Tiers: tiers})
		}
		options = append(options, wabaTopupOption{Product: p, Variants: variantOptions})
	}
	return options
}

// GET /auth/channels/:id
func AuthChannelDetail(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}

	ch, topups, stats, err := loadChannelDetail(c)
	if err != nil {
		return c.String(http.StatusNotFound, "Channel not found")
	}
	if ch.OwnerUserID != current.ID {
		return c.String(http.StatusForbidden, "You don't have access to this channel")
	}

	data := map[string]interface{}{
		"Channel": ch,
		"Topups":  topups,
		"Stats":   stats,
	}

	if ch.Type == "sms" {
		_, variant, ok := smsTopupProduct()
		data["SMSAvailable"] = ok
		if ok {
			data["SMSVariant"] = variant
			price, _ := pricing.EffectivePrice(variant.ID, current.ID)
			data["SMSPrice"] = price
		}
	} else if ch.Type == "waba" {
		data["WABAOptions"] = wabaTopupOptions()
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/my_channel_detail.html", data)
}

// ownedChannel loads the :id channel, distinguishing "doesn't exist" from
// "exists but isn't yours" so callers can 404 vs 403 correctly — shared by
// both topup actions below.
func ownedChannel(c echo.Context, ownerID uint) (ch models.Channel, found, owned bool) {
	if db.DB.First(&ch, c.Param("id")).Error != nil {
		return ch, false, false
	}
	return ch, true, ch.OwnerUserID == ownerID
}

// POST /auth/channels/:id/topup/sms
func AuthTopupSMS(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}
	ch, found, owned := ownedChannel(c, current.ID)
	if !found {
		return c.String(http.StatusNotFound, "Channel not found")
	}
	if !owned {
		return c.String(http.StatusForbidden, "You don't have access to this channel")
	}
	if ch.Type != "sms" {
		return c.String(http.StatusBadRequest, "This channel does not accept SMS topups")
	}

	quantity, err := strconv.ParseInt(c.FormValue("quantity"), 10, 64)
	if err != nil || quantity <= 0 {
		return c.String(http.StatusBadRequest, "A valid quantity is required")
	}

	product, variant, ok := smsTopupProduct()
	if !ok {
		return c.String(http.StatusInternalServerError, "SMS topup is not configured yet")
	}
	if product.MinQuota > 0 && quantity < product.MinQuota {
		return c.String(http.StatusBadRequest, "Quantity is below the minimum topup")
	}

	price, err := pricing.EffectivePrice(variant.ID, current.ID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to resolve price")
	}

	topup := models.ChannelTopup{
		ChannelID:     ch.ID,
		VariantID:     variant.ID,
		Quantity:      quantity,
		Amount:        price * quantity,
		Status:        "pending",
		RequestedByID: current.ID,
	}
	if err := db.DB.Create(&topup).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to submit topup")
	}
	return c.Redirect(http.StatusSeeOther, "/auth/channels/"+c.Param("id"))
}

// POST /auth/channels/:id/topup/waba
func AuthTopupWABA(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}
	ch, found, owned := ownedChannel(c, current.ID)
	if !found {
		return c.String(http.StatusNotFound, "Channel not found")
	}
	if !owned {
		return c.String(http.StatusForbidden, "You don't have access to this channel")
	}
	if ch.Type != "waba" {
		return c.String(http.StatusBadRequest, "This channel does not accept WABA topups")
	}

	variantID, err := strconv.ParseUint(c.FormValue("variant_id"), 10, 64)
	if err != nil || variantID == 0 {
		return c.String(http.StatusBadRequest, "A variant is required")
	}
	var variant models.ProductVariant
	if err := db.DB.First(&variant, variantID).Error; err != nil {
		return c.String(http.StatusBadRequest, "Variant not found")
	}

	var tierID uint
	price, err := pricing.EffectivePrice(variant.ID, current.ID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to resolve price")
	}
	if variant.PricingMode == "tiering" {
		tid, err := strconv.ParseUint(c.FormValue("tier_id"), 10, 64)
		if err != nil || tid == 0 {
			return c.String(http.StatusBadRequest, "A tier is required for this variant")
		}
		var tier models.ProductVariantTier
		if err := db.DB.Where("id = ? AND variant_id = ?", tid, variant.ID).First(&tier).Error; err != nil {
			return c.String(http.StatusBadRequest, "Tier not found")
		}
		tierID = tier.ID
		price = tier.Price
	}

	topup := models.ChannelTopup{
		ChannelID:     ch.ID,
		VariantID:     variant.ID,
		TierID:        tierID,
		Quantity:      1,
		Amount:        price,
		Status:        "pending",
		RequestedByID: current.ID,
	}
	if err := db.DB.Create(&topup).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to submit topup")
	}
	return c.Redirect(http.StatusSeeOther, "/auth/channels/"+c.Param("id"))
}
