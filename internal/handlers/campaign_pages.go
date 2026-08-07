// Client-facing, key-authenticated Campaign/Cart/Invoice pages
// (cart-transaction.md §Campaign page) — hardcoded router->handler->
// template, replacing the old DB-driven /campaign/add Page row. Every page
// here is a thin shell: the handler only resolves the actor and embeds a
// CampaignKey the page's own JS re-attaches to every /campaign/api/* fetch
// (see internal/auth.ResolveKeyActor) — all real data loading happens
// client-side against the JSON API, same as the mockup page it replaces.
package handlers

import (
	"html/template"

	"cms-go/internal/auth"
	"cms-go/internal/config"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// renderCampaignPage renders a campaign view inside either the client's
// minimal layout or (from admin_campaign.go) the admin layout — unlike
// renderWithLayout, it doesn't assume a session (auth.CtxUser), so callers
// populate CurrentUser/CampaignKey/Sandbox themselves. Deliberately does
// NOT blanket-convert string fields to template.HTML the way
// renderWithLayout does for rich Page content: every value here (OrderID,
// CampaignKey, ...) comes from a URL path/query param, so it must stay
// auto-escaped.
func renderCampaignPage(c echo.Context, layoutPath, viewPath string, data map[string]interface{}) error {
	tmpl, err := template.ParseFiles(layoutPath, viewPath)
	if err != nil {
		return err
	}
	if data == nil {
		data = map[string]interface{}{}
	}
	data["SiteName"] = config.SiteName()
	return tmpl.ExecuteTemplate(c.Response().Writer, "layout", data)
}

const clientLayout = "internal/views/campaign/client-layout.html"

func clientPageData(c echo.Context) map[string]interface{} {
	user, sandbox, _ := campaignActor(c)
	return map[string]interface{}{
		"CurrentUser": user,
		"CampaignKey": c.QueryParam("key"),
		"Sandbox":     sandbox,
		"Embedded":    true,
		"BasePath":    "/campaign",
	}
}

// GET /campaign?key=
func CampaignList(c echo.Context) error {
	return renderCampaignPage(c, clientLayout, "internal/views/campaign/list.html", clientPageData(c))
}

// GET /campaign/add?key=
func CampaignAdd(c echo.Context) error {
	data := clientPageData(c)
	data["Mode"] = "add"
	return renderCampaignPage(c, clientLayout, "internal/views/campaign/form.html", data)
}

// GET /campaign/edit/:id?key=
func CampaignEdit(c echo.Context) error {
	data := clientPageData(c)
	data["Mode"] = "edit"
	data["OrderID"] = c.Param("id")
	return renderCampaignPage(c, clientLayout, "internal/views/campaign/form.html", data)
}

// GET /campaign/detail/:id?key=
func CampaignDetail(c echo.Context) error {
	data := clientPageData(c)
	data["OrderID"] = c.Param("id")
	return renderCampaignPage(c, clientLayout, "internal/views/campaign/detail.html", data)
}

// GET /cart?key=
func CartList(c echo.Context) error {
	return renderCampaignPage(c, clientLayout, "internal/views/campaign/cart.html", clientPageData(c))
}

// GET /invoice/:code?key=
func InvoiceDetail(c echo.Context) error {
	data := clientPageData(c)
	data["InvoiceID"] = c.Param("code")
	return renderCampaignPage(c, clientLayout, "internal/views/campaign/invoice.html", data)
}

// campaignKeyForSession ensures the logged-in admin session user has a
// personal API key (generating one on first use, same as
// AuthResetAPIKey/auth.GenerateAPIKey) so the admin campaign pages can
// reuse the exact same client-side JS + /campaign/api/* endpoints as the
// key-authenticated client pages — /admin/campaign is that same composer
// experience for the admin's own account, just wrapped in the admin layout
// and reached via session instead of a URL key.
func campaignKeyForSession(user *models.User) (string, error) {
	if user.APIKey != nil && *user.APIKey != "" {
		return *user.APIKey, nil
	}
	key, err := auth.GenerateAPIKey()
	if err != nil {
		return "", err
	}
	user.APIKey = &key
	return key, db.DB.Model(user).Update("api_key", key).Error
}
