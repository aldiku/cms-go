// Admin-side Campaign pages (/admin/campaign*) — session+RBAC gated
// (registered in the `admin` echo.Group, see router.go), but rendered with
// the exact same content templates and client-side JS/API calls as the
// key-authenticated client pages in campaign_pages.go: the admin's own
// personal API key is transparently embedded into the page so its fetches
// against /campaign/api/* work the same way, just scoped to the admin's own
// account (mirroring how /auth/channels is the session-based counterpart of
// a key-based integration).
package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

const adminLayout = "internal/views/admin/admin-layout.html"

func adminCampaignPageData(c echo.Context) (map[string]interface{}, error) {
	current, ok := currentUser(c)
	if !ok {
		return nil, echo.NewHTTPError(http.StatusUnauthorized)
	}
	key, err := campaignKeyForSession(&current)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"CurrentUser": current,
		"CampaignKey": key,
		"Sandbox":     false,
		"Embedded":    false,
		"BasePath":    "/admin/campaign",
	}, nil
}

// GET /admin/campaign
func AdminCampaignList(c echo.Context) error {
	data, err := adminCampaignPageData(c)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to prepare campaign key")
	}
	return renderCampaignPage(c, adminLayout, "internal/views/campaign/list.html", data)
}

// GET /admin/campaign/add
func AdminCampaignAdd(c echo.Context) error {
	data, err := adminCampaignPageData(c)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to prepare campaign key")
	}
	data["Mode"] = "add"
	return renderCampaignPage(c, adminLayout, "internal/views/campaign/form.html", data)
}

// GET /admin/campaign/edit/:id
func AdminCampaignEdit(c echo.Context) error {
	data, err := adminCampaignPageData(c)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to prepare campaign key")
	}
	data["Mode"] = "edit"
	data["OrderID"] = c.Param("id")
	return renderCampaignPage(c, adminLayout, "internal/views/campaign/form.html", data)
}

// GET /admin/campaign/detail/:id
func AdminCampaignDetail(c echo.Context) error {
	data, err := adminCampaignPageData(c)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to prepare campaign key")
	}
	data["OrderID"] = c.Param("id")
	return renderCampaignPage(c, adminLayout, "internal/views/campaign/detail.html", data)
}
