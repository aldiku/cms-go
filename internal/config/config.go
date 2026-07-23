package config

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

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
