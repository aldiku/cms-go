package notify

import "regexp"

var placeholderRe = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*\}\}`)

// Render replaces "{{key}}" placeholders in body with values from params.
// An unmapped/unknown placeholder is left as-is rather than blanked, so a
// typo'd key or a missing mapping is visible (e.g. in a test send) instead
// of silently disappearing into empty text.
func Render(body string, params map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(body, func(match string) string {
		key := placeholderRe.FindStringSubmatch(match)[1]
		if v, ok := params[key]; ok {
			return v
		}
		return match
	})
}
