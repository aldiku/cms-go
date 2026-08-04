package config

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"cms-go/internal/db"
	"cms-go/internal/models"
)

// generalSetting fetches the singleton GeneralSetting row (see
// internal/handlers/admin_general_settings.go), or a zero-value one if it
// doesn't exist yet (before the first admin save) or the DB isn't connected
// (e.g. called too early in boot).
func generalSetting() models.GeneralSetting {
	var s models.GeneralSetting
	if db.DB != nil {
		db.DB.First(&s, 1)
	}
	return s
}

func RootPath() string {
	projectDirName := os.Getenv("DIR_NAME")
	projectName := regexp.MustCompile(`^(.*` + projectDirName + `)`)
	currentWorkDirectory, _ := os.Getwd()
	rootPath := projectName.Find([]byte(currentWorkDirectory))
	return string(rootPath)
}

// SiteURL returns the configured public site URL (no trailing slash),
// used to build absolute canonical/og:url values for SEO tags.
func SiteURL() string {
	return strings.TrimRight(os.Getenv("SITE_URL"), "/")
}

// SiteName returns the site's display name: the admin-set Site Title
// (General Settings) if configured, else the APP_NAME env, else "CMS".
func SiteName() string {
	if title := generalSetting().SiteTitle; title != "" {
		return title
	}
	if name := os.Getenv("APP_NAME"); name != "" {
		return name
	}
	return "CMS"
}

// SiteTagline returns the admin-set Site Tagline (General Settings), or ""
// if none is configured.
func SiteTagline() string {
	return generalSetting().Tagline
}

// DefaultRegisterRoleID returns the role an admin has configured (General
// Settings) to grant new self-registered users (POST /auth/register). 0
// means unconfigured — callers should fall back to their own default (the
// seeded "member" role, see auth.SeedAuth).
func DefaultRegisterRoleID() uint {
	return generalSetting().DefaultRoleID
}

// GeneratePageLimit returns how many latest-updated pages to pre-render
// during bulk generation (GENERATE_PAGE_LIMIT env, default 50). Pages
// outside this set are generated lazily on first request.
func GeneratePageLimit() int {
	if n, err := strconv.Atoi(os.Getenv("GENERATE_PAGE_LIMIT")); err == nil && n > 0 {
		return n
	}
	return 50
}

// APIKey returns the value that must match the X-API-Key header for any
// live API Builder endpoint tagged "auth". Empty means auth-tagged
// endpoints reject every request (fail closed) — never silently public.
func APIKey() string {
	return os.Getenv("API_KEY")
}

// APIBasePath returns the URL prefix every admin-defined API Builder
// endpoint is publicly reachable under (API_BASE_PATH env, default "/api"),
// with no trailing slash.
func APIBasePath() string {
	p := os.Getenv("API_BASE_PATH")
	if p == "" {
		p = "/api"
	}
	return strings.TrimRight(p, "/")
}

// AppKey returns the passphrase used to encrypt sensitive settings at rest
// (currently: SMTP config passwords — see utils.SimpleEncrypt/SimpleDecrypt).
// Empty means encryption still "works" but with a guessable key, so callers
// should treat empty as a misconfiguration, not silently proceed.
func AppKey() string {
	return os.Getenv("APP_KEY")
}
