package apiengine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CoerceValue converts a raw string (query/form/path/default) or an
// already-decoded JSON value (body — may already be float64/bool/string/
// map/[]interface{}) into the Go type named by typ: "string"|"integer"|
// "float"|"boolean"|"date"|"json".
func CoerceValue(raw interface{}, typ string) (interface{}, error) {
	if raw == nil {
		return nil, nil
	}

	switch typ {
	case "string":
		if s, ok := raw.(string); ok {
			return s, nil
		}
		return fmt.Sprintf("%v", raw), nil

	case "integer":
		switch v := raw.(type) {
		case string:
			n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid integer: %q", v)
			}
			return n, nil
		case float64:
			return int64(v), nil
		case int64:
			return v, nil
		case int:
			return int64(v), nil
		default:
			return nil, fmt.Errorf("cannot coerce %T to integer", v)
		}

	case "float":
		switch v := raw.(type) {
		case string:
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return nil, fmt.Errorf("invalid float: %q", v)
			}
			return f, nil
		case float64:
			return v, nil
		default:
			return nil, fmt.Errorf("cannot coerce %T to float", v)
		}

	case "boolean":
		switch v := raw.(type) {
		case string:
			b, err := strconv.ParseBool(strings.TrimSpace(v))
			if err != nil {
				return nil, fmt.Errorf("invalid boolean: %q", v)
			}
			return b, nil
		case bool:
			return v, nil
		default:
			return nil, fmt.Errorf("cannot coerce %T to boolean", v)
		}

	case "date":
		switch v := raw.(type) {
		case string:
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t, nil
			}
			if t, err := time.Parse("2006-01-02", v); err == nil {
				return t, nil
			}
			return nil, fmt.Errorf("invalid date: %q (expected RFC3339 or YYYY-MM-DD)", v)
		case time.Time:
			return v, nil
		default:
			return nil, fmt.Errorf("cannot coerce %T to date", v)
		}

	case "json":
		switch v := raw.(type) {
		case string:
			var js interface{}
			if err := json.Unmarshal([]byte(v), &js); err != nil {
				return nil, fmt.Errorf("invalid json: %w", err)
			}
			b, _ := json.Marshal(js)
			return string(b), nil
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("cannot marshal to json: %w", err)
			}
			return string(b), nil
		}

	default:
		return nil, fmt.Errorf("unknown parameter type %q", typ)
	}
}
