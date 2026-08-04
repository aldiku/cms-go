// Public JSON auth API — /auth/login, /auth/register, /auth/forgot-password,
// /auth/reset-password (plus /auth/csrf-token to obtain the token these POST
// bodies require). Distinct from auth_handlers.go (HTML admin login) and
// auth_account.go (session-gated self-service pages): every handler here is
// reachable without a session and returns JSON, for a decoupled frontend.
package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	"cms-go/internal/auth"
	"cms-go/internal/config"
	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/notify"

	"github.com/labstack/echo/v4"
)

// authCSRFCookie is a double-submit CSRF cookie for the JSON auth endpoints:
// the frontend fetches a token from GET /auth/csrf-token (which sets this
// cookie), then echoes the same value back as "csrf_token" in the JSON body
// of /auth/login and /auth/register. Deliberately not HttpOnly, same
// rationale as the admin login form's _csrf cookie (see router.go).
const (
	authCSRFCookie = "csrf_token"
	authCSRFTTL    = 30 * time.Minute
)

type userJSON struct {
	ID         uint   `json:"id"`
	Firstname  string `json:"firstname"`
	Lastname   string `json:"lastname"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	ReferralID uint   `json:"referral_id"`
	Verified   bool   `json:"verified"`
}

func toUserJSON(u models.User) userJSON {
	return userJSON{
		ID:         u.ID,
		Firstname:  u.Firstname,
		Lastname:   u.Lastname,
		Email:      u.Email,
		Phone:      u.Phone,
		ReferralID: u.ReferralID,
		Verified:   u.VerifiedAt != nil,
	}
}

// GET /auth/csrf-token — issues a fresh double-submit CSRF token, returned
// in the JSON body and mirrored into a readable cookie so a JS frontend can
// send it back as "csrf_token" on the next login/register POST.
func AuthCSRFToken(c echo.Context) error {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate csrf token"})
	}
	token := hex.EncodeToString(raw)

	c.SetCookie(&http.Cookie{
		Name:     authCSRFCookie,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(authCSRFTTL),
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(config.SiteURL(), "https://"),
	})

	return c.JSON(http.StatusOK, map[string]string{"csrf_token": token})
}

// validCSRF reports whether token matches the value issued via
// AuthCSRFToken and stored in the request's csrf_token cookie.
func validCSRF(c echo.Context, token string) bool {
	cookie, err := c.Cookie(authCSRFCookie)
	if err != nil || cookie.Value == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(token)) == 1
}

type authLoginRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	CSRFToken string `json:"csrf_token"`
}

// POST /auth/login
func AuthAPILogin(c echo.Context) error {
	var req authLoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if !validCSRF(c, req.CSRFToken) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "invalid or missing csrf token"})
	}
	if req.Email == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email and password are required"})
	}

	var user models.User
	err := db.DB.Where("email = ? AND status = 1", req.Email).First(&user).Error
	if err != nil || !auth.CheckPassword(user.Password, req.Password) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
	}

	token, err := auth.CreateSession(user.ID, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "could not start session"})
	}
	auth.SetSessionCookie(c, token)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  toUserJSON(user),
	})
}

type authRegisterRequest struct {
	Firstname  string `json:"firstname"`
	Lastname   string `json:"lastname"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Password   string `json:"password"`
	ReferralID uint   `json:"referral_id"`
	CSRFToken  string `json:"csrf_token"`
}

// POST /auth/register
func AuthAPIRegister(c echo.Context) error {
	var req authRegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if !validCSRF(c, req.CSRFToken) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "invalid or missing csrf token"})
	}
	if req.Email == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email and password are required"})
	}
	if len(req.Password) < 8 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
	}

	var referralID uint
	if req.ReferralID != 0 {
		var referrer models.User
		if err := db.DB.First(&referrer, req.ReferralID).Error; err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid referral_id"})
		}
		referralID = referrer.ID
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to process password"})
	}

	// Self-registered accounts get an admin-configured default role if one
	// is set (General Settings), else the no-permissions "member" role
	// (seeded at boot, see auth.SeedAuth) — role_id has a not-null FK to
	// roles, so this can't be left at 0.
	registerRole := models.Role{ID: config.DefaultRegisterRoleID()}
	if registerRole.ID == 0 || db.DB.First(&registerRole, registerRole.ID).Error != nil {
		if err := db.DB.Where("role = ?", "member").First(&registerRole).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "registration temporarily unavailable"})
		}
	}

	user := models.User{
		Firstname:  req.Firstname,
		Lastname:   req.Lastname,
		Email:      req.Email,
		Phone:      req.Phone,
		Password:   hash,
		RoleID:     registerRole.ID,
		ReferralID: referralID,
		Status:     1,
	}
	if err := db.DB.Create(&user).Error; err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email already registered"})
	}

	verificationURL := ""
	if token, err := auth.CreateVerificationToken(user.ID); err != nil {
		log.Printf("notify: create verification token failed: %v", err)
	} else {
		verificationURL = config.SiteURL() + "/auth/verify?token=" + token
	}

	go func() {
		data := map[string]string{
			"_to":              user.Email,
			"user_name":        user.FullName(),
			"user_email":       user.Email,
			"user_role":        registerRole.Role,
			"site_name":        config.SiteName(),
			"site_url":         config.SiteURL(),
			"verification_url": verificationURL,
		}
		for _, err := range notify.Dispatch("register_user", data) {
			log.Printf("notify: register_user dispatch failed: %v", err)
		}
	}()

	token, err := auth.CreateSession(user.ID, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		return c.JSON(http.StatusCreated, map[string]interface{}{"user": toUserJSON(user)})
	}
	auth.SetSessionCookie(c, token)

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"token": token,
		"user":  toUserJSON(user),
	})
}

type authForgotPasswordRequest struct {
	Email string `json:"email"`
}

// POST /auth/forgot-password — always responds with the same generic
// message whether or not the email is registered, so this endpoint can't be
// used to enumerate accounts. Fires the "password_reset" notification hook
// (see internal/notify/registry.go) with the raw token as reset_token_code
// plus a convenience reset_url, both derived from the same token.
func AuthAPIForgotPassword(c echo.Context) error {
	var req authForgotPasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.Email == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email is required"})
	}

	const genericMsg = "If an account exists for this email, a password reset link has been sent."

	var user models.User
	if err := db.DB.Where("email = ? AND status = 1", req.Email).First(&user).Error; err != nil {
		return c.JSON(http.StatusOK, map[string]string{"message": genericMsg})
	}

	tokenCode, err := auth.CreateResetToken(user.ID)
	if err != nil {
		log.Printf("notify: create reset token failed: %v", err)
		return c.JSON(http.StatusOK, map[string]string{"message": genericMsg})
	}

	go func() {
		data := map[string]string{
			"_to":              user.Email,
			"user_name":        user.FullName(),
			"user_email":       user.Email,
			"reset_token_code": tokenCode,
			"reset_url":        config.SiteURL() + "/reset-password?token=" + tokenCode,
		}
		for _, err := range notify.Dispatch("password_reset", data) {
			log.Printf("notify: password_reset dispatch failed: %v", err)
		}
	}()

	return c.JSON(http.StatusOK, map[string]string{"message": genericMsg})
}

type authResetPasswordRequest struct {
	ResetTokenCode  string `json:"reset_token_code"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// POST /auth/reset-password — consumes the one-time token from
// forgot-password, sets the new password, and revokes every existing
// session for that user so a stolen session can't outlive the reset.
func AuthAPIResetPassword(c echo.Context) error {
	var req authResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.ResetTokenCode == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "reset_token_code and password are required"})
	}
	if len(req.Password) < 8 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
	}
	if req.Password != req.ConfirmPassword {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "password and confirm_password do not match"})
	}

	user, err := auth.ConsumeResetToken(req.ResetTokenCode)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid or expired reset token"})
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to process password"})
	}
	if err := db.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("password", hash).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update password"})
	}

	db.DB.Delete(&models.Session{}, "user_id = ?", user.ID)

	return c.JSON(http.StatusOK, map[string]string{"message": "password has been reset successfully"})
}
