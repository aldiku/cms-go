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
