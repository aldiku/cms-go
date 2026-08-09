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
		"Title":        "WordPress Migration",
		"RolledBack":   c.QueryParam("rolled_back") == "1",
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

// GET /admin/migrate-wordpress/rollback - shows migrated posts and rollback confirmation
func AdminWPMigrateRollback(c echo.Context) error {
	migratedPosts, err := migrator.GetMigratedPosts()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to fetch migrated posts: "+err.Error())
	}

	data := map[string]interface{}{
		"MigratedPosts": migratedPosts,
		"Count":         len(migratedPosts),
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/migrate_wordpress_rollback.html", data)
}

// POST /admin/migrate-wordpress/rollback/execute - executes the rollback
func AdminWPMigrateRollbackExecute(c echo.Context) error {
	if err := migrator.RollbackWPData(); err != nil {
		return c.String(http.StatusInternalServerError, "Rollback failed: "+err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/admin/migrate-wordpress?rolled_back=1")
}
