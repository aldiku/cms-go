package server

import (
	"cms-go/internal/generator"
	"errors"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// NewRenderer builds the Echo renderer used for admin forms and the 404 page.
func NewRenderer() *Template {
	tmpl, err := generator.ParseTemplates()
	if err != nil {
		log.Printf("template parse error: %v", err)
		tmpl = template.New("")
	}

	return &Template{templates: tmpl}
}

// DynamicPage serves a page's static HTML, treating the generated pages dir
// as a cache: serve the file if it exists, otherwise look the slug up in the
// DB, render + cache it via generator.GeneratePage, and serve that. Only the
// GENERATE_PAGE_LIMIT latest-updated pages are pre-rendered in bulk, so any
// page outside that set gets generated here on first request. Unknown slugs
// (and render failures) get the 404 page.
func DynamicPage(c echo.Context) error {
	path := c.Request().URL.Path
	if strings.Contains(path, "..") { // never treat traversal as a page
		return c.Render(http.StatusNotFound, "404.html", nil)
	}

	if content, err := os.ReadFile(generator.PageFilePath(path)); err == nil {
		return c.HTMLBlob(http.StatusOK, content)
	}

	content, err := generator.GeneratePage(path)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("lazy page generation failed for %s: %v", path, err)
		}
		return c.Render(http.StatusNotFound, "404.html", nil)
	}
	return c.HTMLBlob(http.StatusOK, content)
}
