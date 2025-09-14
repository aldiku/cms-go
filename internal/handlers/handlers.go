package handlers

import (
	"cms-go/internal/db"
	"cms-go/internal/models"
	"net/http"

	"github.com/labstack/echo/v4"
)

func AdminDashboard(c echo.Context) error {
	return c.String(http.StatusOK, "Admin Dashboard")
}

func AdminPages(c echo.Context) error {
	var pages []models.Page
	db.DB.Find(&pages)

	return c.Render(http.StatusOK, "pages.html", map[string]interface{}{
		"Pages": pages,
	})
}

func AdminMenus(c echo.Context) error {
	return c.String(http.StatusOK, "Menu management (DB version)")
}
