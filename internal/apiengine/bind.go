// Package apiengine executes admin-defined SQL endpoints (the Low-Code API
// Builder): rewriting ":name" placeholders into driver-bound positional
// args, resolving request parameters from query/body/form/path, running the
// query, and shaping the JSON response. Parameter values are always bound
// as real query args — never string-concatenated into SQL text — which is
// the actual defense against SQL injection through parameter values. There
// is no defense against an endpoint author writing dangerous SQL
// themselves; that's an accepted, explicit product tradeoff (see the plan).
package apiengine

import (
	"fmt"
	"unicode"

	"cms-go/internal/models"
)

// ParsePlaceholders scans sqlText for ":name" tokens (word-chars) and
// rewrites each into "?" (GORM's dialect-neutral placeholder — the Postgres
// dialector converts "?" to "$1,$2..." itself at build time; we never
// hand-build "$N" strings). Postgres "::type" casts are left untouched
// since a token preceded by another ':' is not treated as a placeholder
// start. Returns the rewritten SQL plus the ordered list of param names
// referenced, one entry per occurrence (duplicates preserved).
//
// Known limitation: this doesn't understand SQL string literals, so a
// literal ":word"-looking sequence inside a quoted string would be
// misparsed as a placeholder. No SQL parser dependency exists in this repo;
// documented as a v1 limitation in the SQL Editor tab's hint text.
func ParsePlaceholders(sqlText string) (rewritten string, occurrences []string) {
	runes := []rune(sqlText)
	n := len(runes)

	var sb []rune
	i := 0
	for i < n {
		c := runes[i]
		if c == ':' {
			if i+1 < n && runes[i+1] == ':' {
				sb = append(sb, ':', ':')
				i += 2
				continue
			}
			if i+1 < n && (unicode.IsLetter(runes[i+1]) || runes[i+1] == '_') {
				j := i + 1
				for j < n && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_') {
					j++
				}
				occurrences = append(occurrences, string(runes[i+1:j]))
				sb = append(sb, '?')
				i = j
				continue
			}
		}
		sb = append(sb, c)
		i++
	}
	return string(sb), occurrences
}

// BindArgs resolves each name in occurrences against already-resolved
// values (see ResolveParamValues — coercion/defaults happen there, not
// here), returning the final args slice in the same order (so a repeated
// placeholder duplicates the same value). Errors if the SQL references a
// name that isn't a declared parameter at all.
func BindArgs(occurrences []string, params []models.ApiParameter, values map[string]interface{}) ([]interface{}, error) {
	declared := make(map[string]bool, len(params))
	for _, p := range params {
		declared[p.Name] = true
	}

	args := make([]interface{}, 0, len(occurrences))
	for _, name := range occurrences {
		if !declared[name] {
			return nil, fmt.Errorf("sql references undeclared parameter %q", name)
		}
		args = append(args, values[name])
	}
	return args, nil
}
