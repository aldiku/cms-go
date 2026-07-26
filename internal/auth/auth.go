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
	SessionCookie = "cms_session"
	sessionTTL    = 7 * 24 * time.Hour
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
