// Public self-service Channel registration — POST /auth/channels/register
// and its supporting GET /auth/channels/waba-products. Registered as
// standalone routes (not inside the `account` group in router.go) because
// that group's auth.RequireAuth middleware redirects unauthenticated
// requests to the HTML login page, which is useless to a fetch()-driven
// JSON/multipart form on a public page — this file does the same session
// check by hand and replies with JSON instead. Matches the pattern already
// used for the JSON auth API (auth_api.go).
package handlers

import (
	"encoding/json"
	"net/http"

	"cms-go/internal/auth"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// currentUserJSON resolves the session cookie the same way
// auth.RequireAuth does, but returns ok=false instead of redirecting so
// JSON/fetch-based callers get a clean 401 rather than a login page body.
func currentUserJSON(c echo.Context) (models.User, bool) {
	cookie, err := c.Cookie(auth.SessionCookie)
	if err != nil || cookie.Value == "" {
		return models.User{}, false
	}
	user, err := auth.UserFromToken(cookie.Value)
	if err != nil {
		return models.User{}, false
	}
	return user, true
}

// GET /auth/channels/waba-products — public catalog lookup (no
// registration action, nothing sensitive) so the WABA registration page's
// product cards (with a tiering "session package" picker where applicable)
// can be populated from real Product/ProductVariant/ProductVariantTier
// rows instead of a hardcoded list. Reuses wabaTopupOptions
// (auth_channels.go) so the registration estimate and the post-activation
// topup picker never disagree about what a product costs.
func ChannelWABAProductsJSON(c echo.Context) error {
	current, loggedIn := currentUserJSON(c)

	options := wabaTopupOptions()
	out := make([]map[string]interface{}, 0, len(options))
	for _, opt := range options {
		out = append(out, wabaProductJSON(opt, current.ID, loggedIn))
	}
	return c.JSON(http.StatusOK, out)
}

// wabaProductJSON builds one product's registration-card payload,
// following the pricing hierarchy: the product's own base Price (no
// variant configured yet) → its first active variant's price (fixed:
// Price; tiering: the full tier list, so the page can offer a session
// package <select> instead of collapsing to one number) → a logged-in
// caller's PriceOverride on that variant. Overrides only apply to fixed
// pricing — there's no per-tier override support yet (PriceOverride is
// keyed by variant, not tier).
func wabaProductJSON(opt wabaTopupOption, userID uint, loggedIn bool) map[string]interface{} {
	out := map[string]interface{}{
		"code":         opt.Product.Code,
		"name":         opt.Product.Name,
		"description":  opt.Product.Description,
		"pricing_mode": "", // "fixed" | "tiering" | "" (no active variant configured yet)
		"price":        opt.Product.Price,
		"variant_name": "", // fixed-mode subtitle, e.g. "One time charge + first 3 Months"
		"is_custom":    false,
		"tiers":        []map[string]interface{}{},
	}
	if len(opt.Variants) == 0 {
		return out
	}

	v := opt.Variants[0]
	out["pricing_mode"] = v.Variant.PricingMode
	out["variant_name"] = v.Variant.Name

	if v.Variant.PricingMode != "fixed" {
		tiers := make([]map[string]interface{}, 0, len(v.Tiers))
		for _, t := range v.Tiers {
			tiers = append(tiers, map[string]interface{}{
				"id": t.ID, "label": t.Label, "price": t.Price,
				"quantity": t.Quantity, "is_custom": t.IsCustom,
			})
		}
		out["tiers"] = tiers
		out["price"] = 0
		return out
	}

	price := v.Variant.Price
	isCustom := false
	if loggedIn {
		var override models.PriceOverride
		if db.DB.Where("variant_id = ? AND target_user_id = ?", v.Variant.ID, userID).First(&override).Error == nil {
			price, isCustom = override.Price, true
		}
	}
	out["price"] = price
	out["is_custom"] = isCustom
	return out
}

// POST /auth/channels/register — public self-service registration for the
// dedicated SMS/WhatsApp pages (note.md §6.1/§6.2). multipart/form-data so
// it can carry the registration document, same fields as the admin create
// form (see buildChannelFromRegistrationForm, admin_channels.go). Starts
// "pending" — unlike admin-created channels, these need review before use.
func RegisterChannel(c echo.Context) error {
	current, ok := currentUserJSON(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Please log in first"})
	}

	ch, err := buildChannelFromRegistrationForm(c, current.ID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	ch.OwnerUserID = current.ID
	ch.Status = "pending"

	// product_selections is the JSON snapshot built client-side by the
	// /register-whatsapp card picker (checked products + quantity + the
	// price shown at submission time) — admin-only registration forms
	// don't send it, so this stays empty for those. Only kept if it's
	// well-formed JSON; a malformed value is dropped rather than failing
	// the whole registration over a cosmetic field.
	if raw := c.FormValue("product_selections"); raw != "" {
		var probe []map[string]interface{}
		if json.Unmarshal([]byte(raw), &probe) == nil {
			ch.InitialProductSelections = raw
		}
	}

	if err := db.DB.Create(&ch).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to submit registration"})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"message": "Registration submitted — an admin will review it shortly.",
		"channel": map[string]interface{}{
			"id":        ch.ID,
			"type":      ch.Type,
			"sender_id": ch.SenderID,
			"status":    ch.Status,
		},
	})
}
