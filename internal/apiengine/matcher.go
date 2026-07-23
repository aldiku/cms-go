package apiengine

import (
	"strings"

	"cms-go/internal/models"

	"gorm.io/gorm"
)

// MatchPath compares a registered pattern ("/users/:id/posts") against an
// incoming path segment-by-segment (split on "/", leading/trailing slashes
// ignored). Literal segments must match exactly; a ":name" segment matches
// any single non-empty segment and is captured. Different segment counts
// never match — there are no wildcard/catch-all segments in v1.
func MatchPath(pattern, path string) (map[string]string, bool) {
	pSegs := splitPath(pattern)
	aSegs := splitPath(path)
	if len(pSegs) != len(aSegs) {
		return nil, false
	}

	params := map[string]string{}
	for i, seg := range pSegs {
		if strings.HasPrefix(seg, ":") {
			if aSegs[i] == "" {
				return nil, false
			}
			params[strings.TrimPrefix(seg, ":")] = aSegs[i]
			continue
		}
		if seg != aSegs[i] {
			return nil, false
		}
	}
	return params, true
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return []string{}
	}
	return strings.Split(p, "/")
}

// FindEndpoint loads active endpoints for method and returns the first
// whose FullPath() matches path via MatchPath, plus the captured path
// params. No in-memory cache in v1 — at expected endpoint-table sizes this
// is sub-millisecond and dominated by the DB round trip either way.
func FindEndpoint(gdb *gorm.DB, method, path string) (models.ApiEndpoint, map[string]string, bool) {
	var candidates []models.ApiEndpoint
	gdb.Where("status = 1 AND method = ?", method).Find(&candidates)

	for _, ep := range candidates {
		if params, ok := MatchPath(ep.FullPath(), path); ok {
			return ep, params, true
		}
	}
	return models.ApiEndpoint{}, nil, false
}
