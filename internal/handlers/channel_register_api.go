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
// product cards can be populated from real Product/ProductVariant rows
// instead of a hardcoded list. Reuses wabaTopupOptions (auth_channels.go)
// for pricing so the registration estimate and the post-activation topup
// picker never disagree about what a product costs. If the caller is
// logged in and has a models.PriceOverride on the matched variant (see
// AuthSetClientPrice/AdminSetPriceOverride), that price is shown instead
// of the catalog default — same "override wins" convention as
// pricing.EffectivePrice, applied here to whichever variant a product
// actually resolves to (fixed or the cheapest tiering bracket).
func ChannelWABAProductsJSON(c echo.Context) error {
	current, loggedIn := currentUserJSON(c)

	options := wabaTopupOptions()
	out := make([]map[string]interface{}, 0, len(options))
	for _, opt := range options {
		price, mode, isCustom := wabaProductDisplayPrice(opt, current.ID, loggedIn)
		out = append(out, map[string]interface{}{
			"code":         opt.Product.Code,
			"name":         opt.Product.Name,
			"price":        price,
			"pricing_mode": mode,     // "fixed" | "tiering" | "" (no active variant configured yet)
			"is_custom":    isCustom, // true when `price` is this user's PriceOverride, not the catalog default
		})
	}
	return c.JSON(http.StatusOK, out)
}

// wabaProductDisplayPrice picks one representative price for a product's
// registration card: a logged-in caller's PriceOverride on the variant if
// one exists, else the first active fixed variant's price, else the
// cheapest non-custom tier of the first active tiering variant (labeled
// "starting from" client-side) — the exact tier/variant is finalized by
// an admin during review, same as the post-activation topup flow. Products
// with no active variant configured yet return price 0 and an empty mode
// so the page can show "price to be confirmed" instead of blocking
// registration on incomplete catalog setup.
func wabaProductDisplayPrice(opt wabaTopupOption, userID uint, loggedIn bool) (price int64, mode string, isCustom bool) {
	for _, v := range opt.Variants {
		if loggedIn {
			var override models.PriceOverride
			if db.DB.Where("variant_id = ? AND target_user_id = ?", v.Variant.ID, userID).First(&override).Error == nil {
				return override.Price, v.Variant.PricingMode, true
			}
		}
		if v.Variant.PricingMode == "fixed" {
			return v.Variant.Price, "fixed", false
		}
		cheapest := int64(-1)
		for _, t := range v.Tiers {
			if t.IsCustom {
				continue
			}
			if cheapest == -1 || t.Price < cheapest {
				cheapest = t.Price
			}
		}
		if cheapest != -1 {
			return cheapest, "tiering", false
		}
	}
	return 0, "", false
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
