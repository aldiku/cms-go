package handlers

import (
	"net/http"

	"cms-go/internal/auth"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// GET /admin/login
func AdminLoginForm(c echo.Context) error {
	// Already logged in? Straight to the panel.
	if cookie, err := c.Cookie(auth.SessionCookie); err == nil && cookie.Value != "" {
		if _, err := auth.UserFromToken(cookie.Value); err == nil {
			return c.Redirect(http.StatusFound, "/admin")
		}
	}
	return c.Render(http.StatusOK, "login-admin.html", map[string]interface{}{})
}

// POST /admin/login
func AdminLogin(c echo.Context) error {
	email := c.FormValue("email")
	password := c.FormValue("password")

	var user models.User
	err := db.DB.Where("email = ? AND status = 1", email).First(&user).Error
	if err != nil || !auth.CheckPassword(user.Password, password) {
		return c.Render(http.StatusUnauthorized, "login-admin.html", map[string]interface{}{
			"Error": "Invalid email or password",
			"Email": email,
		})
	}

	token, err := auth.CreateSession(user.ID)
	if err != nil {
		return c.Render(http.StatusInternalServerError, "login-admin.html", map[string]interface{}{
			"Error": "Could not start session, please try again",
			"Email": email,
		})
	}

	auth.SetSessionCookie(c, token)
	return c.Redirect(http.StatusFound, "/admin")
}

// POST /admin/logout
func AdminLogout(c echo.Context) error {
	if cookie, err := c.Cookie(auth.SessionCookie); err == nil && cookie.Value != "" {
		auth.DestroySession(cookie.Value)
	}
	auth.ClearSessionCookie(c)
	return c.Redirect(http.StatusFound, "/admin/login")
}
