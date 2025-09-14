package server

import (
	"cms-go/internal/db"
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
	db.DB.AutoMigrate(&models.Page{}, &models.Layout{}, &models.Menu{}, &models.Component{})
	// generate templates from DB into views/generated
	if err := GenerateTemplatesFromDB(); err != nil {
		fmt.Println("template generation error:", err)
	}
	e.Renderer = NewRenderer()

	// Admin panel (HTML forms)
	e.GET("/admin", handlers.AdminDashboard)
	e.GET("/admin/pages", handlers.AdminPages)
	e.POST("/admin/pages/new", handlers.AdminCreatePage)
	e.GET("/admin/pages/:id/edit", handlers.AdminEditPage)
	e.POST("/admin/pages/:id/edit", handlers.AdminUpdatePage)

	// Layouts
	e.GET("/admin/layouts", handlers.AdminLayouts)
	e.POST("/admin/layouts/new", handlers.AdminCreateLayout)
	e.GET("/admin/layouts/:id/edit", handlers.AdminEditLayout)
	e.POST("/admin/layouts/:id/edit", handlers.AdminEditLayout)

	// Components
	e.GET("/admin/components", handlers.AdminComponents)
	e.GET("/admin/components/new", handlers.AdminNewComponent)
	e.POST("/admin/components/new", handlers.AdminCreateComponent)
	e.GET("/admin/components/:id/edit", handlers.AdminEditComponent)
	e.POST("/admin/components/:id/edit", handlers.AdminUpdateComponent)

	e.GET("/admin/menus", handlers.AdminMenus)

	// Public frontend routes
	e.Static("/assets", "assets")
	e.GET("/*", DynamicPage)

	return e
}
