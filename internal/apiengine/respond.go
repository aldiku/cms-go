package apiengine

import (
	"net/http"

	"cms-go/internal/models"
)

// BuildResponse shapes rows/execErr into the envelope cfg describes and
// picks the HTTP status. Deliberately does not accept executed SQL text as
// an input at all, so there's no code path that could ever leak it into a
// live public response — that's only ever shown by the admin-only Test
// tool. Errors always come back as a small JSON object regardless of
// envelope, since callers need a discoverable shape to detect failure;
// only the success shape varies by envelope.
func BuildResponse(cfg models.ResponseConfig, rows []map[string]interface{}, execErr error, elapsedMs int64) (int, interface{}) {
	if execErr != nil {
		msg := "internal server error"
		switch {
		case cfg.ErrorMode == "detailed":
			msg = execErr.Error()
		case cfg.ErrorMessage != "":
			msg = cfg.ErrorMessage
		}
		return http.StatusInternalServerError, map[string]interface{}{"success": false, "error": msg}
	}

	shaped := applyFieldRenames(rows, cfg.FieldRenames)

	if len(shaped) == 0 {
		switch cfg.EmptyMode {
		case "null":
			return http.StatusOK, wrapEnvelope(cfg, nil, 0, elapsedMs)
		case "custom_message":
			msg := cfg.EmptyMessage
			if msg == "" {
				msg = "no results"
			}
			return http.StatusOK, map[string]interface{}{"success": true, "message": msg}
		default: // "empty_array"
			return http.StatusOK, wrapEnvelope(cfg, []map[string]interface{}{}, 0, elapsedMs)
		}
	}

	var data interface{} = shaped
	if cfg.SingleRow && len(shaped) == 1 {
		data = shaped[0]
	}
	return http.StatusOK, wrapEnvelope(cfg, data, len(shaped), elapsedMs)
}

func applyFieldRenames(rows []map[string]interface{}, renames map[string]string) []map[string]interface{} {
	if len(renames) == 0 {
		return rows
	}
	out := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		renamed := make(map[string]interface{}, len(row))
		for k, v := range row {
			if newKey, ok := renames[k]; ok && newKey != "" {
				renamed[newKey] = v
			} else {
				renamed[k] = v
			}
		}
		out[i] = renamed
	}
	return out
}

// wrapEnvelope shapes successful data per cfg.Envelope:
//   - "raw": data itself, unwrapped (a bare array or object) — timing can't
//     be attached since there's no wrapper object, so IncludeTiming is
//     ignored in this mode.
//   - "custom": {success, <SuccessField or "data">: data, count[, query_time_ms]}
//   - "data" (default): {success, data, count[, query_time_ms]}
func wrapEnvelope(cfg models.ResponseConfig, data interface{}, count int, elapsedMs int64) interface{} {
	switch cfg.Envelope {
	case "raw":
		if data == nil {
			return []map[string]interface{}{}
		}
		return data
	case "custom":
		field := cfg.SuccessField
		if field == "" {
			field = "data"
		}
		body := map[string]interface{}{"success": true, field: data, "count": count}
		if cfg.IncludeTiming {
			body["query_time_ms"] = elapsedMs
		}
		return body
	default: // "data"
		body := map[string]interface{}{"success": true, "data": data, "count": count}
		if cfg.IncludeTiming {
			body["query_time_ms"] = elapsedMs
		}
		return body
	}
}
