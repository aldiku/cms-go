// Self-service account pages (/auth/profile, /auth/setting) — every
// authenticated user needs these regardless of their role's menu
// permissions, so they're registered outside the RBAC-gated /admin group
// (auth.RequireAuth only, never auth.RequirePermission).
package handlers

import (
	"net/http"
	"time"

	"cms-go/internal/auth"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

func currentUser(c echo.Context) (models.User, bool) {
	u, ok := c.Get(auth.CtxUser).(models.User)
	return u, ok
}

func bindProfileFromForm(c echo.Context, user *models.User) {
	user.Firstname = c.FormValue("firstname")
	user.Lastname = c.FormValue("lastname")
	user.Email = c.FormValue("email")
	user.Phone = c.FormValue("phone")
	user.Address = c.FormValue("address")
	user.Company = c.FormValue("company")
	user.EmployeeID = c.FormValue("employee_id")
	user.Avatar = c.FormValue("avatar")
}

// GET /auth/profile
func AuthProfileForm(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}
	var user models.User
	if err := db.DB.First(&user, current.ID).Error; err != nil {
		return c.Redirect(http.StatusFound, "/admin/login")
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/profile.html", map[string]interface{}{
		"Profile": user,
		"Saved":   c.QueryParam("saved") == "1",
	})
}

// POST /auth/profile
func AuthUpdateProfile(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}
	var user models.User
	if err := db.DB.First(&user, current.ID).Error; err != nil {
		return c.Redirect(http.StatusFound, "/admin/login")
	}

	bindProfileFromForm(c, &user)
	if user.Firstname == "" || user.Email == "" {
		return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/profile.html", map[string]interface{}{
			"Profile": user,
			"Error":   "First name and email are required",
		})
	}

	if err := db.DB.Save(&user).Error; err != nil {
		return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/profile.html", map[string]interface{}{
			"Profile": user,
			"Error":   "Failed to update profile (email may already be in use)",
		})
	}

	return c.Redirect(http.StatusSeeOther, "/auth/profile?saved=1")
}

// POST /auth/profile/password — requires and verifies the current password
// first (unlike admin_users.go's admin-reset flow, which intentionally has
// no such check since an admin resetting someone else's password can't know
// their old one).
func AuthChangePassword(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}
	var user models.User
	if err := db.DB.First(&user, current.ID).Error; err != nil {
		return c.Redirect(http.StatusFound, "/admin/login")
	}

	fail := func(msg string) error {
		return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/profile.html", map[string]interface{}{
			"Profile":       user,
			"PasswordError": msg,
		})
	}

	currentPassword := c.FormValue("current_password")
	newPassword := c.FormValue("new_password")
	confirmPassword := c.FormValue("confirm_password")

	if !auth.CheckPassword(user.Password, currentPassword) {
		return fail("Current password is incorrect")
	}
	if len(newPassword) < 8 {
		return fail("New password must be at least 8 characters")
	}
	if newPassword != confirmPassword {
		return fail("New password and confirmation do not match")
	}

	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fail("Failed to set new password")
	}
	user.Password = hash
	if err := db.DB.Save(&user).Error; err != nil {
		return fail("Failed to save new password")
	}

	return c.Redirect(http.StatusSeeOther, "/auth/profile?saved=1")
}

// sessionView is the display shape for one row of the sessions table —
// Browser/OS are parsed from the stored raw UserAgent at render time only,
// never persisted.
type sessionView struct {
	Token     string
	IPAddress string
	Browser   string
	OS        string
	CreatedAt time.Time
	ExpiresAt time.Time
	IsCurrent bool
}

// GET /auth/setting
func AuthSettingsForm(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}
	var user models.User
	if err := db.DB.First(&user, current.ID).Error; err != nil {
		return c.Redirect(http.StatusFound, "/admin/login")
	}

	var sessions []models.Session
	db.DB.Where("user_id = ?", user.ID).Order("created_at desc").Find(&sessions)

	currentToken := ""
	if cookie, err := c.Cookie(auth.SessionCookie); err == nil {
		currentToken = cookie.Value
	}

	views := make([]sessionView, 0, len(sessions))
	for _, s := range sessions {
		browser, os := auth.ParseUserAgent(s.UserAgent)
		views = append(views, sessionView{
			Token:     s.Token,
			IPAddress: s.IPAddress,
			Browser:   browser,
			OS:        os,
			CreatedAt: s.CreatedAt,
			ExpiresAt: s.ExpiresAt,
			IsCurrent: s.Token == currentToken,
		})
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/settings.html", map[string]interface{}{
		"Sessions":      views,
		"APIKey":        user.APIKey,
		"APIKeySandbox": user.APIKeySandbox,
		"Reset":         c.QueryParam("reset") == "1",
	})
}

// POST /auth/setting/sessions/:token/revoke — only ever deletes a session
// that belongs to the current user (never trust the token path param
// alone). Revoking the caller's own current session logs them out
// immediately instead of redirecting back to a page they can no longer see.
func AuthRevokeSession(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}
	token := c.Param("token")

	db.DB.Delete(&models.Session{}, "token = ? AND user_id = ?", token, current.ID)

	if cookie, err := c.Cookie(auth.SessionCookie); err == nil && cookie.Value == token {
		auth.ClearSessionCookie(c)
		return c.Redirect(http.StatusSeeOther, "/admin/login")
	}
	return c.Redirect(http.StatusSeeOther, "/auth/setting")
}

// POST /auth/setting/sessions/revoke-others
func AuthRevokeOtherSessions(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}
	token := ""
	if cookie, err := c.Cookie(auth.SessionCookie); err == nil {
		token = cookie.Value
	}
	db.DB.Delete(&models.Session{}, "user_id = ? AND token <> ?", current.ID, token)
	return c.Redirect(http.StatusSeeOther, "/auth/setting")
}

// POST /auth/setting/api-key/reset — same operation whether this is the
// user's first key or a reset of an existing one; the UI just labels the
// button differently depending on whether APIKey is already set.
func AuthResetAPIKey(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}
	var user models.User
	if err := db.DB.First(&user, current.ID).Error; err != nil {
		return c.Redirect(http.StatusFound, "/admin/login")
	}

	key, err := auth.GenerateAPIKey()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to generate API key")
	}
	user.APIKey = &key
	if err := db.DB.Save(&user).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to save API key")
	}

	return c.Redirect(http.StatusSeeOther, "/auth/setting?reset=1")
}

// POST /auth/setting/api-key-sandbox/reset — same as AuthResetAPIKey, but
// for models.User.APIKeySandbox: the key that flags a /campaign,
// /cart, /invoice request as sandbox instead of production (see
// internal/auth/keyauth.go's ResolveKeyActor).
func AuthResetAPIKeySandbox(c echo.Context) error {
	current, ok := currentUser(c)
	if !ok {
		return c.Redirect(http.StatusFound, "/admin/login")
	}
	var user models.User
	if err := db.DB.First(&user, current.ID).Error; err != nil {
		return c.Redirect(http.StatusFound, "/admin/login")
	}

	key, err := auth.GenerateAPIKey()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to generate sandbox API key")
	}
	user.APIKeySandbox = &key
	if err := db.DB.Save(&user).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to save sandbox API key")
	}

	return c.Redirect(http.StatusSeeOther, "/auth/setting?reset=1")
}
