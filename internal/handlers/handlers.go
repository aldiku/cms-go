package handlers

import (
	"html/template"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

func AdminDashboard(c echo.Context) error {
	return c.String(http.StatusOK, "Admin Dashboard")
}

func AdminMenus(c echo.Context) error {
	return c.String(http.StatusOK, "Menu management (DB version)")
}

func renderWithLayout(w io.Writer, layoutPath, viewPath string, data map[string]interface{}) error {
	// Parse both layout and view
	tmpl, err := template.ParseFiles(layoutPath, viewPath)
	if err != nil {
		return err
	}

	// Wrap HTML fields to prevent escaping
	for k, v := range data {
		if str, ok := v.(string); ok {
			data[k] = template.HTML(str)
		}
	}

	// Execute the layout template (layout.html should have {{ template "content" . }})
	return tmpl.ExecuteTemplate(w, "layout", data)
}
