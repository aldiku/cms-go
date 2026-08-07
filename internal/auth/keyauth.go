package auth

import (
	"net/http"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// Context keys set by ResolveKeyActor.
const (
	CtxCampaignUser    = "campaignUser"
	CtxCampaignSandbox = "campaignSandbox"
	CtxCampaignSource  = "campaignSource"
)

// ResolveKeyActor authenticates the embeddable /campaign, /cart and
// /invoice surfaces (and their JSON APIs) via a per-user key instead of a
// session cookie — these pages are meant to be embedded as an iframe on a
// reseller's own site, so a cross-origin session cookie isn't available.
// The key is read from the "key" query param (matching the spec's
// {url}/campaign?key={userkey}) or, for pure API integrations, the
// X-API-Key header. Matching models.User.APIKey flags the request as
// production; matching models.User.APIKeySandbox flags it as sandbox — see
// cart-transaction.md's "userkey ... jadi definisi flag sandbox/prod mode".
func ResolveKeyActor(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		key := c.QueryParam("key")
		if key == "" {
			key = c.Request().Header.Get("X-API-Key")
		}
		if key == "" {
			return c.String(http.StatusUnauthorized, "Missing key")
		}

		var user models.User
		sandbox := false
		if err := db.DB.Where("api_key = ? AND status = 1", key).First(&user).Error; err == nil {
			sandbox = false
		} else if err := db.DB.Where("api_key_sandbox = ? AND status = 1", key).First(&user).Error; err == nil {
			sandbox = true
		} else {
			return c.String(http.StatusUnauthorized, "Invalid key")
		}

		source := c.QueryParam("source")
		if source == "" {
			source = c.Request().Header.Get("X-Source")
		}
		if source == "" {
			source = "web"
		}

		c.Set(CtxCampaignUser, user)
		c.Set(CtxCampaignSandbox, sandbox)
		c.Set(CtxCampaignSource, source)
		return next(c)
	}
}
