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
// "Initial Product" dropdown can be populated from real Product rows
// instead of a hardcoded list, same convention as the topup flow
// (auth_channels.go's wabaTopupOptions).
func ChannelWABAProductsJSON(c echo.Context) error {
	var products []models.Product
	db.DB.Where("is_campaignable = ? AND code <> ?", false, "smstopup").Order("name asc").Find(&products)

	out := make([]map[string]interface{}, 0, len(products))
	for _, p := range products {
		out = append(out, map[string]interface{}{"code": p.Code, "name": p.Name})
	}
	return c.JSON(http.StatusOK, out)
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
