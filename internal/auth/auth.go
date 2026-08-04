// Package auth provides password hashing, DB-backed cookie sessions, and the
// RBAC middleware protecting the admin panel. Access rules live in the
// roles/menus/permissions tables: a role gets per-menu create/read/update/
// delete flags, and admin routes are matched to menus by path prefix.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"cms-go/internal/config"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

const (
	SessionCookie   = "cms_session"
	sessionTTL      = 7 * 24 * time.Hour
	verificationTTL = 24 * time.Hour
	resetTTL        = 1 * time.Hour
)

func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(hash), err
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// CreateSession stores a new session row for the user (with the request's IP
// and User-Agent, shown later on the account settings page) and returns its
// token.
func CreateSession(userID uint, ip, userAgent string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)

	session := models.Session{
		Token:     token,
		UserID:    userID,
		IPAddress: ip,
		UserAgent: userAgent,
		ExpiresAt: time.Now().Add(sessionTTL),
	}
	if err := db.DB.Create(&session).Error; err != nil {
		return "", err
	}
	return token, nil
}

func DestroySession(token string) {
	db.DB.Delete(&models.Session{}, "token = ?", token)
}

// CreateVerificationToken stores a new one-time email-verification token for
// userID and returns it. Any previous unconsumed token for the same user is
// removed first, so only the most recently sent verification link works.
func CreateVerificationToken(userID uint) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)

	db.DB.Delete(&models.EmailVerification{}, "user_id = ?", userID)
	verification := models.EmailVerification{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(verificationTTL),
	}
	if err := db.DB.Create(&verification).Error; err != nil {
		return "", err
	}
	return token, nil
}

// ConsumeVerificationToken validates token, deletes it (one-time use), and
// returns the user it belonged to. The caller is responsible for setting
// User.VerifiedAt.
func ConsumeVerificationToken(token string) (models.User, error) {
	var user models.User

	var verification models.EmailVerification
	if err := db.DB.First(&verification, "token = ?", token).Error; err != nil {
		return user, errors.New("invalid verification link")
	}
	db.DB.Delete(&verification)

	if time.Now().After(verification.ExpiresAt) {
		return user, errors.New("verification link has expired")
	}

	if err := db.DB.Preload("Role").First(&user, verification.UserID).Error; err != nil {
		return user, err
	}
	return user, nil
}

// CreateResetToken stores a new one-time password-reset token for userID and
// returns it. Any previous unconsumed token for the same user is removed
// first, so only the most recently requested reset link works.
func CreateResetToken(userID uint) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)

	db.DB.Delete(&models.PasswordReset{}, "user_id = ?", userID)
	reset := models.PasswordReset{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(resetTTL),
	}
	if err := db.DB.Create(&reset).Error; err != nil {
		return "", err
	}
	return token, nil
}

// ConsumeResetToken validates token, deletes it (one-time use), and returns
// the user it belonged to. The caller is responsible for setting the new
// password.
func ConsumeResetToken(token string) (models.User, error) {
	var user models.User

	var reset models.PasswordReset
	if err := db.DB.First(&reset, "token = ?", token).Error; err != nil {
		return user, errors.New("invalid reset token")
	}
	db.DB.Delete(&reset)

	if time.Now().After(reset.ExpiresAt) {
		return user, errors.New("reset token has expired")
	}

	if err := db.DB.First(&user, reset.UserID).Error; err != nil {
		return user, err
	}
	return user, nil
}

// GenerateAPIKey returns a new 32-character random hex string for
// models.User.APIKey — same crypto/rand + hex.EncodeToString pattern as
// session tokens, just half the byte length (16 bytes -> 32 hex chars).
func GenerateAPIKey() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// ParseUserAgent best-effort parses a raw User-Agent header into a short
// human-readable browser and OS label for display on the sessions list.
// Deliberately not a full UA-parsing library — no such dependency exists in
// this repo, and this only needs to be good enough for "which of my devices
// is this", not analytics-grade accuracy.
func ParseUserAgent(ua string) (browser, os string) {
	browser = "Unknown browser"
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/"), strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Chrome/"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	}

	os = "Unknown OS"
	switch {
	case strings.Contains(ua, "Windows"):
		os = "Windows"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		os = "macOS"
	case strings.Contains(ua, "Android"):
		os = "Android"
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"):
		os = "iOS"
	case strings.Contains(ua, "Linux"):
		os = "Linux"
	}
	return browser, os
}

// UserFromToken resolves a session token to its active user (role preloaded).
func UserFromToken(token string) (models.User, error) {
	var user models.User

	var session models.Session
	if err := db.DB.First(&session, "token = ?", token).Error; err != nil {
		return user, err
	}
	if time.Now().After(session.ExpiresAt) {
		DestroySession(token)
		return user, errors.New("session expired")
	}

	if err := db.DB.Preload("Role").First(&user, session.UserID).Error; err != nil {
		return user, err
	}
	if user.Status != 1 {
		return user, errors.New("user inactive")
	}
	return user, nil
}

// SetSessionCookie writes the session cookie on the response. Secure is set
// when the configured SITE_URL is https.
func SetSessionCookie(c echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(sessionTTL),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(config.SiteURL(), "https://"),
	})
}

func ClearSessionCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
