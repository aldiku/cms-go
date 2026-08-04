package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"cms-go/internal/db"
	"cms-go/internal/generator"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// getGeneralSetting loads the singleton row (id=1), creating it empty if it
// doesn't exist yet.
func getGeneralSetting() models.GeneralSetting {
	var s models.GeneralSetting
	db.DB.FirstOrCreate(&s, models.GeneralSetting{ID: 1})
	return s
}

// GET /admin/general-settings
func AdminGeneralSettings(c echo.Context) error {
	var setting models.GeneralSetting
	db.DB.Preload("Favicon").FirstOrCreate(&setting, models.GeneralSetting{ID: 1})

	var roles []models.Role
	db.DB.Order("role asc").Find(&roles)

	data := map[string]interface{}{
		"Setting": setting,
		"Roles":   roles,
		"Saved":   c.QueryParam("saved") == "1",
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/general_settings.html", data)
}

// POST /admin/general-settings
func AdminUpdateGeneralSettings(c echo.Context) error {
	setting := getGeneralSetting()

	setting.SiteTitle = c.FormValue("site_title")
	setting.Tagline = c.FormValue("tagline")

	if roleIDStr := c.FormValue("default_role_id"); roleIDStr != "" {
		if roleID, err := strconv.ParseUint(roleIDStr, 10, 64); err == nil {
			setting.DefaultRoleID = uint(roleID)
		}
	} else {
		setting.DefaultRoleID = 0
	}

	if faviconIDStr := c.FormValue("favicon_id"); faviconIDStr != "" {
		if faviconID, err := strconv.ParseUint(faviconIDStr, 10, 64); err == nil {
			setting.FaviconID = uint(faviconID)
		}
	} else {
		setting.FaviconID = 0
	}

	if err := db.DB.Save(&setting).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to save settings")
	}

	// Every layout's <link rel="icon"> already points at the static,
	// well-known "/assets/favicon.ico" path — copying the chosen media's
	// file there means the new favicon takes effect everywhere without
	// having to template that link across every layout.
	if setting.FaviconID != 0 {
		var favicon models.Media
		if err := db.DB.First(&favicon, setting.FaviconID).Error; err == nil {
			copyFaviconFile(favicon.Path)
		}
	}

	if err := generator.GenerateTemplatesFromDB(); err != nil {
		return c.String(http.StatusInternalServerError, "Settings saved, but failed to regenerate pages: "+err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/admin/general-settings?saved=1")
}

func copyFaviconFile(mediaPath string) {
	src, err := os.Open(filepath.Join("assets", mediaPath))
	if err != nil {
		return
	}
	defer src.Close()

	dst, err := os.Create(filepath.Join("assets", "favicon.ico"))
	if err != nil {
		return
	}
	defer dst.Close()

	io.Copy(dst, src)
}
