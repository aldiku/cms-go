package server

import (
	"bytes"
	"cms-go/internal/db"
	"cms-go/internal/handlers"
	"cms-go/internal/models"
	"fmt"
	"html/template"
	"io"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// RenderComponent mencoba render component, skip kalau tidak ada template
func RenderComponent(t *template.Template, name string, data interface{}) template.HTML {
	var buf bytes.Buffer
	err := t.ExecuteTemplate(&buf, "component-"+name, data)
	if err != nil {
		// skip jika template tidak ditemukan
		return template.HTML("")
	}
	return template.HTML(buf.String())
}

func NewRenderer() *Template {
	// bikin root template + daftarkan func di awal
	root := template.New("").Funcs(template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"renderComponent": func(name string, data interface{}) template.HTML {
			// nanti kita inject tmpl di bawah
			return template.HTML("")
		},
	})

	// parse semua template file (layout, base, dsb.)
	tmpl := template.Must(root.ParseGlob("internal/views/**/*.html"))

	// load components dari DB
	var comps []models.Component
	if err := db.DB.Find(&comps).Error; err == nil {
		for _, c := range comps {
			_, err := tmpl.Parse(c.Template)
			if err != nil {
				panic(fmt.Sprintf("error parsing component %s: %v", c.Name, err))
			}
		}
	}

	// override renderComponent supaya pakai tmpl final
	tmpl.Funcs(template.FuncMap{
		"renderComponent": func(name string, data interface{}) template.HTML {
			return RenderComponent(tmpl, name, data)
		},
	})

	return &Template{templates: tmpl}
}

func New() *echo.Echo {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// init DB
	db.Connect()
	db.DB.AutoMigrate(&models.Page{}, &models.Layout{}, &models.Menu{}, &models.Component{})

	e.Renderer = NewRenderer()

	// Public frontend routes
	e.GET("/*", handlers.DynamicPage)

	// Admin panel (HTML forms)
	e.GET("/admin", handlers.AdminDashboard)
	e.GET("/admin/pages", handlers.AdminPages)
	e.POST("/admin/pages/new", handlers.AdminCreatePage)
	e.GET("/admin/pages/:id/edit", handlers.AdminEditPage)
	e.POST("/admin/pages/:id/edit", handlers.AdminUpdatePage)

	// Layouts
	e.GET("/admin/layouts", handlers.AdminLayouts)
	e.POST("/admin/layouts/new", handlers.AdminCreateLayout)

	// Components
	e.GET("/admin/components", handlers.AdminComponents)
	e.GET("/admin/components/new", handlers.AdminNewComponent)
	e.POST("/admin/components/new", handlers.AdminCreateComponent)
	e.GET("/admin/components/:id/edit", handlers.AdminEditComponent)
	e.POST("/admin/components/:id/edit", handlers.AdminUpdateComponent)

	e.GET("/admin/menus", handlers.AdminMenus)

	return e
}
