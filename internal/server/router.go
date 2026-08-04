package server

import (
	"cms-go/internal/auth"
	"cms-go/internal/config"
	"cms-go/internal/db"
	"cms-go/internal/generator"
	"cms-go/internal/handlers"
	"cms-go/internal/models"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

// migratePageDefaults runs idempotent one-time backfills needed by the
// WordPress-style Page fields (Author/Status/Type) added after pages already
// existed in the DB. Safe to run on every boot.
func migratePageDefaults() {
	// Pre-existing rows predate the Status column: treat them as already
	// published so they stay publicly visible instead of silently 404ing.
	db.DB.Model(&models.Page{}).
		Where("status = '' OR status IS NULL").
		Update("status", models.PageStatusPublish)

	// "page"/"post" used to be rendered via the JSON page-builder schema
	// (rows/columns/components) by default; they're now plain HTML content,
	// like "html". Retag any row whose content is still that JSON schema as
	// "builder" so it keeps rendering through the old code path.
	var candidates []models.Page
	db.DB.Where("type IN ?", []string{"page", "post"}).Find(&candidates)
	for _, p := range candidates {
		var schema map[string]interface{}
		if err := json.Unmarshal([]byte(p.Content), &schema); err != nil {
			continue
		}
		if _, ok := schema["rows"]; !ok {
			continue
		}
		db.DB.Model(&models.Page{}).Where("id = ?", p.ID).Update("type", "builder")
	}

	// The page editor template used to wrap {{.Page.Content}} in extra
	// indentation/newlines, which the Ace editor picked up as literal
	// leading/trailing whitespace on every load. Trim whatever that already
	// baked into saved rows; new saves are trimmed at the handler level.
	var allPages []models.Page
	db.DB.Select("id", "content").Find(&allPages)
	for _, p := range allPages {
		if trimmed := strings.TrimSpace(p.Content); trimmed != p.Content {
			db.DB.Model(&models.Page{}).Where("id = ?", p.ID).Update("content", trimmed)
		}
	}
}

func New() *echo.Echo {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// init DB
	db.Connect()
	db.DB.AutoMigrate(
		&models.Page{}, &models.Layout{}, &models.MenuGroup{}, &models.Menu{}, &models.Component{},
		&models.User{}, &models.Role{}, &models.Permission{}, &models.Session{},
		&models.Revision{}, &models.ApiEndpoint{}, &models.Category{}, &models.Tag{},
		&models.Media{}, &models.SMTPConfig{}, &models.EmailTemplate{}, &models.NotificationHook{},
		&models.EmailVerification{}, &models.PasswordReset{},
		&models.ProductCategory{}, &models.Product{}, &models.ProductVariant{}, &models.ProductVariantTier{},
		&models.PriceOverride{}, &models.GeneralSetting{},
	)
	auth.SeedAuth()
	auth.SeedAuthPages()
	migratePageDefaults()
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
	e.GET("/auth/verify", handlers.AuthVerifyEmail)

	// JSON auth API — public, for a decoupled frontend (login/register use
	// the double-submit csrf_token from GET /auth/csrf-token; reuses the
	// same rate limiter shape as the admin login form).
	authAPIRateLimit := middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
			Rate:      rate.Limit(5.0 / 60.0),
			Burst:     5,
			ExpiresIn: 3 * time.Minute,
		}),
		IdentifierExtractor: func(c echo.Context) (string, error) {
			return c.RealIP(), nil
		},
	})
	e.GET("/auth/csrf-token", handlers.AuthCSRFToken)
	e.POST("/auth/login", handlers.AuthAPILogin, authAPIRateLimit)
	e.POST("/auth/register", handlers.AuthAPIRegister, authAPIRateLimit)
	e.POST("/auth/forgot-password", handlers.AuthAPIForgotPassword, authAPIRateLimit)
	e.POST("/auth/reset-password", handlers.AuthAPIResetPassword, authAPIRateLimit)

	// Self-service account pages — session required, but NOT RBAC-gated by
	// menu permission: every logged-in user needs their own profile/settings
	// regardless of what their role's menus allow.
	account := e.Group("/auth", auth.RequireAuth)
	account.GET("/profile", handlers.AuthProfileForm)
	account.POST("/profile", handlers.AuthUpdateProfile)
	account.POST("/profile/password", handlers.AuthChangePassword)
	account.GET("/setting", handlers.AuthSettingsForm)
	account.POST("/setting/sessions/:token/revoke", handlers.AuthRevokeSession)
	account.POST("/setting/sessions/revoke-others", handlers.AuthRevokeOtherSessions)
	account.POST("/setting/api-key/reset", handlers.AuthResetAPIKey)
	account.GET("/pricing", handlers.AuthPricing)
	account.POST("/pricing/set", handlers.AuthSetClientPrice)

	// Admin panel (HTML forms) — session required, RBAC per menu
	admin := e.Group("/admin", auth.RequireAuth, auth.RequirePermission)
	admin.GET("", handlers.AdminDashboard)

	// Pages
	admin.GET("/pages", handlers.AdminPages)
	admin.GET("/pages/new", handlers.AdminPageEditor)
	admin.POST("/pages/new", handlers.AdminCreatePage)
	admin.GET("/pages/:id/edit", handlers.AdminPageEditor)
	admin.POST("/pages/:id/edit", handlers.AdminUpdatePage)

	// Categories & Tags (post taxonomy)
	admin.GET("/categories", handlers.AdminCategories)
	admin.GET("/categories/new", handlers.AdminCategoryForm)
	admin.POST("/categories/new", handlers.AdminCreateCategory)
	admin.GET("/categories/:id/edit", handlers.AdminCategoryForm)
	admin.POST("/categories/:id/edit", handlers.AdminUpdateCategory)
	admin.POST("/categories/:id/delete", handlers.AdminDeleteCategory)

	admin.GET("/tags", handlers.AdminTags)
	admin.GET("/tags/new", handlers.AdminTagForm)
	admin.POST("/tags/new", handlers.AdminCreateTag)
	admin.GET("/tags/:id/edit", handlers.AdminTagForm)
	admin.POST("/tags/:id/edit", handlers.AdminUpdateTag)
	admin.POST("/tags/:id/delete", handlers.AdminDeleteTag)

	// Product Categories
	admin.GET("/product-categories", handlers.AdminProductCategories)
	admin.GET("/product-categories/new", handlers.AdminProductCategoryForm)
	admin.POST("/product-categories/new", handlers.AdminCreateProductCategory)
	admin.GET("/product-categories/:id/edit", handlers.AdminProductCategoryForm)
	admin.POST("/product-categories/:id/edit", handlers.AdminUpdateProductCategory)
	admin.POST("/product-categories/:id/delete", handlers.AdminDeleteProductCategory)

	// Products (hierarchical tree, like Menus)
	admin.GET("/products", handlers.AdminProducts)
	admin.POST("/products/new", handlers.AdminCreateProduct)
	admin.POST("/products/:id/edit", handlers.AdminUpdateProduct)
	admin.POST("/products/:id/delete", handlers.AdminDeleteProduct)
	admin.POST("/products/reorder", handlers.AdminReorderProducts)

	// Product Variants (scoped to a Product node)
	admin.GET("/products/variants/json", handlers.AdminVariantsJSON)
	admin.GET("/products/:id/variants", handlers.AdminProductVariants)
	admin.POST("/products/:id/variants/new", handlers.AdminCreateProductVariant)
	admin.POST("/products/:id/variants/:variant_id/edit", handlers.AdminUpdateProductVariant)
	admin.POST("/products/:id/variants/:variant_id/delete", handlers.AdminDeleteProductVariant)

	// Custom Pricing (per-variant, per-user price overrides)
	admin.POST("/products/:id/variants/:variant_id/pricing/new", handlers.AdminSetPriceOverride)
	admin.POST("/products/:id/variants/:variant_id/pricing/:override_id/delete", handlers.AdminDeletePriceOverride)
	admin.GET("/custom-pricing", handlers.AdminCustomPricingList)
	admin.GET("/custom-pricing/user/:user_id", handlers.AdminCustomPricingForUser)
	admin.POST("/custom-pricing/user/:user_id/set", handlers.AdminSetPriceOverrideForUser)
	admin.POST("/custom-pricing/:override_id/delete", handlers.AdminDeleteCustomPricing)

	// Media Library — image/video/audio/document/archive uploads
	admin.GET("/medias", handlers.AdminMedias)
	admin.GET("/medias/json", handlers.AdminMediasJSON)
	admin.POST("/medias/upload", handlers.AdminMediaUpload, middleware.BodyLimit("50M"))
	admin.POST("/medias/:id/delete", handlers.AdminDeleteMedia)

	// General Settings (Settings > General Settings)
	admin.GET("/general-settings", handlers.AdminGeneralSettings)
	admin.POST("/general-settings", handlers.AdminUpdateGeneralSettings)

	// Email SMTP (Settings > Email SMTP)
	admin.GET("/smtp", handlers.AdminSMTPConfigs)
	admin.GET("/smtp/new", handlers.AdminSMTPConfigForm)
	admin.POST("/smtp/new", handlers.AdminCreateSMTPConfig)
	admin.GET("/smtp/:id/edit", handlers.AdminSMTPConfigForm)
	admin.POST("/smtp/:id/edit", handlers.AdminUpdateSMTPConfig)
	admin.POST("/smtp/:id/delete", handlers.AdminDeleteSMTPConfig)
	admin.POST("/smtp/:id/test", handlers.AdminTestSMTPConfig)

	// Email Templates
	admin.GET("/email-templates", handlers.AdminEmailTemplates)
	admin.GET("/email-templates/new", handlers.AdminEmailTemplateForm)
	admin.POST("/email-templates/new", handlers.AdminCreateEmailTemplate)
	admin.GET("/email-templates/:id/edit", handlers.AdminEmailTemplateForm)
	admin.POST("/email-templates/:id/edit", handlers.AdminUpdateEmailTemplate)
	admin.POST("/email-templates/:id/delete", handlers.AdminDeleteEmailTemplate)

	// Notification Manager — binds a hook (see internal/notify.Registry) to
	// an SMTP config + email template + field mapping
	admin.GET("/notification-hooks", handlers.AdminNotificationHooks)
	admin.GET("/notification-hooks/new", handlers.AdminNotificationHookForm)
	admin.POST("/notification-hooks/new", handlers.AdminCreateNotificationHook)
	admin.GET("/notification-hooks/:id/edit", handlers.AdminNotificationHookForm)
	admin.POST("/notification-hooks/:id/edit", handlers.AdminUpdateNotificationHook)
	admin.POST("/notification-hooks/:id/delete", handlers.AdminDeleteNotificationHook)
	admin.POST("/notification-hooks/:id/test", handlers.AdminTestNotificationHook)

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
	admin.GET("/users/json", handlers.AdminUsersJSON)
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
	admin.POST("/menus/new", handlers.AdminCreateMenu)
	admin.POST("/menus/:id/edit", handlers.AdminUpdateMenu)
	admin.POST("/menus/:id/delete", handlers.AdminDeleteMenu)
	admin.POST("/menus/reorder", handlers.AdminReorderMenus)
	admin.POST("/menus/groups", handlers.AdminCreateMenuGroup)
	admin.POST("/menus/groups/:id/delete", handlers.AdminDeleteMenuGroup)

	// Revision history (read-only detail view; shared by pages/layouts/components)
	admin.GET("/revisions/:id", handlers.AdminViewRevision)

	// API Builder — split-view editor uses JSON request bodies, not form posts
	admin.GET("/api-builder", handlers.AdminAPIBuilder)
	admin.GET("/api-builder/:id/json", handlers.AdminAPIEndpointJSON)
	admin.POST("/api-builder/new", handlers.AdminCreateAPIEndpoint)
	admin.POST("/api-builder/:id/edit", handlers.AdminUpdateAPIEndpoint)
	admin.POST("/api-builder/:id/delete", handlers.AdminDeleteAPIEndpoint)
	admin.POST("/api-builder/test", handlers.AdminTestAPIEndpoint)

	// DB Manager — native Postgres table browser/editor + SQL console
	admin.GET("/db-manager", handlers.AdminDBManager)
	admin.GET("/db-manager/tables/:table/json", handlers.AdminDBManagerTableJSON)
	admin.GET("/db-manager/tables/:table/rows", handlers.AdminDBManagerBrowseRows)
	admin.POST("/db-manager/tables/:table/rows/new", handlers.AdminDBManagerInsertRow)
	admin.POST("/db-manager/tables/:table/rows/edit", handlers.AdminDBManagerUpdateRow)
	admin.POST("/db-manager/tables/:table/rows/delete", handlers.AdminDBManagerDeleteRow)
	admin.POST("/db-manager/sql", handlers.AdminDBManagerRunSQL)

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
