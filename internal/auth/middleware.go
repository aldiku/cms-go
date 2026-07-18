package auth

import (
	"net/http"
	"strings"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// Context keys set by RequireAuth and read by handlers/renderWithLayout.
const (
	CtxUser     = "authUser"
	CtxNavMenus = "authNavMenus"
)

// SuperadminRole bypasses permission checks so the permission editor can
// never lock everyone out.
const SuperadminRole = "superadmin"

// RequireAuth resolves the session cookie to a user or redirects to the
// login page. It also loads the menus the user's role can read (the
// sidebar's data) into the context so every admin render gets them.
func RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cookie, err := c.Cookie(SessionCookie)
		if err != nil || cookie.Value == "" {
			return c.Redirect(http.StatusFound, "/admin/login")
		}

		user, err := UserFromToken(cookie.Value)
		if err != nil {
			ClearSessionCookie(c)
			return c.Redirect(http.StatusFound, "/admin/login")
		}

		c.Set(CtxUser, user)
		c.Set(CtxNavMenus, readableMenus(user))
		return next(c)
	}
}

// readableMenus returns the active menus the user's role has read access to,
// ordered for the sidebar. Superadmin sees every active menu.
func readableMenus(user models.User) []models.Menu {
	var menus []models.Menu
	q := db.DB.Model(&models.Menu{}).Where("menus.status = 1").Order("menus.list_order ASC")
	if user.Role.Role != SuperadminRole {
		q = q.Joins(`JOIN permissions ON permissions.menu_id = menus.id AND permissions.role_id = ? AND permissions."read" = true`, user.RoleID)
	}
	q.Find(&menus)
	return menus
}

// RequirePermission enforces the role's CRUD flags for the menu matching the
// request path (longest path-prefix wins, so /admin/pages/5/edit maps to the
// /admin/pages menu). Verb mapping: GET needs read; POST needs delete when
// the path ends in /delete, create when it ends in /new, update otherwise.
func RequirePermission(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, ok := c.Get(CtxUser).(models.User)
		if !ok {
			return c.Redirect(http.StatusFound, "/admin/login")
		}
		if user.Role.Role == SuperadminRole {
			return next(c)
		}

		path := c.Request().URL.Path
		menu, found := menuForPath(path)
		if !found {
			return renderForbidden(c)
		}

		var perm models.Permission
		if err := db.DB.Where("role_id = ? AND menu_id = ?", user.RoleID, menu.ID).First(&perm).Error; err != nil {
			return renderForbidden(c)
		}

		allowed := false
		switch c.Request().Method {
		case http.MethodGet:
			allowed = perm.CanRead
		case http.MethodPost:
			switch {
			case strings.HasSuffix(path, "/delete"):
				allowed = perm.CanDelete
			case strings.HasSuffix(path, "/new"):
				allowed = perm.CanCreate
			default:
				allowed = perm.CanUpdate
			}
		}

		if !allowed {
			return renderForbidden(c)
		}
		return next(c)
	}
}

// menuForPath finds the active menu whose path is the longest prefix of the
// request path (on segment boundaries, so /admin/pages doesn't match
// /admin/pages-x).
func menuForPath(path string) (models.Menu, bool) {
	var menus []models.Menu
	db.DB.Where("status = 1").Find(&menus)

	var best models.Menu
	bestLen := -1
	for _, m := range menus {
		if m.Path == "" {
			continue
		}
		if path == m.Path || strings.HasPrefix(path, m.Path+"/") {
			if len(m.Path) > bestLen {
				best = m
				bestLen = len(m.Path)
			}
		}
	}
	return best, bestLen >= 0
}

func renderForbidden(c echo.Context) error {
	return c.Render(http.StatusForbidden, "403.html", nil)
}
