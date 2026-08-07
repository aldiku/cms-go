package handlers

import (
	"net/http"

	"cms-go/internal/auth"
	"cms-go/internal/migrator"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// GET /admin/migrate-wordpress - displays the migration control panel
func AdminMigrateWordPress(c echo.Context) error {
	data := map[string]interface{}{
		"Title": "WordPress Migration",
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/migrate_wordpress.html", data)
}

// POST /admin/migrate-wordpress/run - executes the WordPress migration
func AdminMigrateWordPressRun(c echo.Context) error {
	var defaultAuthorID uint
	if user, ok := c.Get(auth.CtxUser).(models.User); ok {
		defaultAuthorID = user.ID
	}

	if err := migrator.MigrateWPData(defaultAuthorID); err != nil {
		return c.String(http.StatusInternalServerError, "Migration failed: "+err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/admin/pages")
}
