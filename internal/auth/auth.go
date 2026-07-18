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

// CreateSession stores a new session row for the user and returns its token.
func CreateSession(userID uint) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)

	session := models.Session{
		Token:     token,
		UserID:    userID,
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
