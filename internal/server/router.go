package server

import (
	"cms-go/internal/auth"
	"cms-go/internal/db"
	"cms-go/internal/generator"
	"cms-go/internal/handlers"
	"cms-go/internal/models"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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
	)
	auth.SeedAuth()
	// generate templates from DB into views/generated
	if err := generator.GenerateTemplatesFromDB(); err != nil {
		fmt.Println("template generation error:", err)
	}
	e.Renderer = NewRenderer()

	// Auth (public)
	e.GET("/admin/login", handlers.AdminLoginForm)
	e.POST("/admin/login", handlers.AdminLogin)
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

	// Permissions (matrix editor per role)
	admin.GET("/permissions", handlers.AdminPermissions)
	admin.POST("/permissions", handlers.AdminSavePermissions)

	// Public frontend routes
	e.Static("/assets", "assets")
	e.GET("/*", DynamicPage)

	return e
}
