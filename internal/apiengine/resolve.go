package apiengine

import (
	"fmt"

	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// ResolveParamValues extracts + coerces one value per declared parameter
// from its declared Source (query/body/form/path), falling back to Default
// when absent, erroring when a required parameter has neither. bodyJSON is
// the (already read once — see ReadJSONBody) decoded JSON body, or nil if
// no param uses "body".
func ResolveParamValues(c echo.Context, params []models.ApiParameter, bodyJSON map[string]interface{}, pathParams map[string]string) (map[string]interface{}, error) {
	values := make(map[string]interface{}, len(params))

	for _, p := range params {
		var raw interface{}
		found := false

		switch p.Source {
		case "query":
			if v := c.QueryParam(p.Name); v != "" {
				raw, found = v, true
			}
		case "form":
			if v := c.FormValue(p.Name); v != "" {
				raw, found = v, true
			}
		case "path":
			if v, ok := pathParams[p.Name]; ok {
				raw, found = v, true
			}
		case "body":
			if v, ok := bodyJSON[p.Name]; ok {
				raw, found = v, true
			}
		default:
			return nil, fmt.Errorf("parameter %q has unknown source %q", p.Name, p.Source)
		}

		if !found {
			if p.Required {
				return nil, fmt.Errorf("missing required parameter %q", p.Name)
			}
			if p.Default == "" {
				values[p.Name] = nil
				continue
			}
			raw = p.Default
		}

		coerced, err := CoerceValue(raw, p.Type)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", p.Name, err)
		}
		values[p.Name] = coerced
	}

	return values, nil
}

// ResolveTestValues resolves bind values for the admin-only Test tool: an
// explicit test value per parameter name if provided, else the parameter's
// Default, else an error if required. Unlike ResolveParamValues, this never
// reads from an echo.Context — the "request" here is the in-editor draft,
// not a live HTTP request.
func ResolveTestValues(params []models.ApiParameter, testValues map[string]interface{}) (map[string]interface{}, error) {
	values := make(map[string]interface{}, len(params))
	for _, p := range params {
		raw, found := testValues[p.Name]
		if !found || raw == nil {
			if p.Default == "" {
				if p.Required {
					return nil, fmt.Errorf("missing required parameter %q", p.Name)
				}
				values[p.Name] = nil
				continue
			}
			raw = p.Default
		}
		coerced, err := CoerceValue(raw, p.Type)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", p.Name, err)
		}
		values[p.Name] = coerced
	}
	return values, nil
}
