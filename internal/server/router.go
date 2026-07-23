package server

import (
	"cms-go/internal/auth"
	"cms-go/internal/config"
	"cms-go/internal/db"
	"cms-go/internal/generator"
	"cms-go/internal/handlers"
	"cms-go/internal/models"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

func New() *echo.Echo {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// init DB
	db.Connect()
	db.DB.AutoMigrate(
		&models.Page{}, &models.Layout{}, &models.Menu{}, &models.Component{},
		&models.User{}, &models.Role{}, &models.Permission{}, &models.Session{},
		&models.Revision{}, &models.ApiEndpoint{},
	)
	auth.SeedAuth()
	// generate templates from DB into views/generated
	if err := generator.GenerateTemplatesFromDB(); err != nil {
		fmt.Println("template generation error:", err)
	}

	e.Renderer = NewRenderer()

	// CSRF for the login form: double-submit cookie, token read from the
	// _csrf form field. The cookie is deliberately not HttpOnly and scoped to
	// "/" so a CMS-built /login page can copy it into its hidden field via JS.
	loginCSRF := middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLookup:    "form:_csrf",
		CookiePath:     "/",
		CookieSameSite: http.SameSiteLaxMode,
		CookieSecure:   strings.HasPrefix(config.SiteURL(), "https://"),
	})

	// Rate limit login attempts per IP: burst of 5, then ~1 attempt per 12s.
	loginRateLimit := middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
			Rate:      rate.Limit(5.0 / 60.0),
			Burst:     5,
			ExpiresIn: 3 * time.Minute,
		}),
		IdentifierExtractor: func(c echo.Context) (string, error) {
			return c.RealIP(), nil
		},
	})

	// Auth (public)
	e.GET("/admin/login", handlers.AdminLoginForm, loginCSRF)
	e.POST("/admin/login", handlers.AdminLogin, loginRateLimit, loginCSRF)
	e.POST("/admin/logout", handlers.AdminLogout)

	// Admin panel (HTML forms) — session required, RBAC per menu
	admin := e.Group("/admin", auth.RequireAuth, auth.RequirePermission)
	admin.GET("", handlers.AdminDashboard)

	// Pages
	admin.GET("/pages", handlers.AdminPages)
	admin.GET("/pages/new", handlers.AdminPageEditor)
	admin.POST("/pages/new", handlers.AdminCreatePage)
	admin.GET("/pages/:id/edit", handlers.AdminPageEditor)
	admin.POST("/pages/:id/edit", handlers.AdminUpdatePage)

	// Layouts
	admin.GET("/layouts", handlers.AdminLayouts)
	admin.POST("/layouts/new", handlers.AdminCreateLayout)
	admin.GET("/layouts/:id/edit", handlers.AdminEditLayout)
	admin.POST("/layouts/:id/edit", handlers.AdminEditLayout)

	// Components
	admin.GET("/components", handlers.AdminComponents)
	admin.GET("/components/new", handlers.AdminNewComponent)
	admin.POST("/components/new", handlers.AdminCreateComponent)
	admin.GET("/components/:id/edit", handlers.AdminEditComponent)
	admin.POST("/components/:id/edit", handlers.AdminUpdateComponent)

	// Users
	admin.GET("/users", handlers.AdminUsers)
	admin.GET("/users/new", handlers.AdminUserForm)
	admin.POST("/users/new", handlers.AdminCreateUser)
	admin.GET("/users/:id/edit", handlers.AdminUserForm)
	admin.POST("/users/:id/edit", handlers.AdminUpdateUser)
	admin.POST("/users/:id/delete", handlers.AdminDeleteUser)

	// Roles
	admin.GET("/roles", handlers.AdminRoles)
	admin.GET("/roles/new", handlers.AdminRoleForm)
	admin.POST("/roles/new", handlers.AdminCreateRole)
	admin.GET("/roles/:id/edit", handlers.AdminRoleForm)
	admin.POST("/roles/:id/edit", handlers.AdminUpdateRole)
	admin.POST("/roles/:id/delete", handlers.AdminDeleteRole)

	// Menus
	admin.GET("/menus", handlers.AdminMenus)
	admin.GET("/menus/new", handlers.AdminMenuForm)
	admin.POST("/menus/new", handlers.AdminCreateMenu)
	admin.GET("/menus/:id/edit", handlers.AdminMenuForm)
	admin.POST("/menus/:id/edit", handlers.AdminUpdateMenu)
	admin.POST("/menus/:id/delete", handlers.AdminDeleteMenu)

	// Revision history (read-only detail view; shared by pages/layouts/components)
	admin.GET("/revisions/:id", handlers.AdminViewRevision)

	// API Builder — split-view editor uses JSON request bodies, not form posts
	admin.GET("/api-builder", handlers.AdminAPIBuilder)
	admin.GET("/api-builder/:id/json", handlers.AdminAPIEndpointJSON)
	admin.POST("/api-builder/new", handlers.AdminCreateAPIEndpoint)
	admin.POST("/api-builder/:id/edit", handlers.AdminUpdateAPIEndpoint)
	admin.POST("/api-builder/:id/delete", handlers.AdminDeleteAPIEndpoint)
	admin.POST("/api-builder/test", handlers.AdminTestAPIEndpoint)

	// Permissions (matrix editor per role)
	admin.GET("/permissions", handlers.AdminPermissions)
	admin.POST("/permissions", handlers.AdminSavePermissions)

	// File Manager
	admin.GET("/file-manager", handlers.AdminFileManager)
	admin.GET("/file-manager/edit/*", handlers.AdminFileEdit)
	admin.POST("/file-manager/save", handlers.AdminFileSave)
	admin.POST("/file-manager/delete", handlers.AdminFileDelete)

	// Public frontend routes
	e.Static("/assets", "assets")
	e.Any(config.APIBasePath()+"/*", handlers.PublicAPIDispatch)
	e.GET("/*", DynamicPage)

	return e
}
