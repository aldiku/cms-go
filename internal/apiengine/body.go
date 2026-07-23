package apiengine

import (
	"encoding/json"
	"io"

	"github.com/labstack/echo/v4"
)

const ctxJSONBody = "apiengine.jsonBody"

// ReadJSONBody reads and decodes the request body into a map at most once
// per request — Echo's request body is a single-use io.Reader, and an
// endpoint can declare several "body"-sourced parameters that all need to
// share one read — caching the result on c via c.Set. Returns an empty
// (non-nil) map for an empty body.
func ReadJSONBody(c echo.Context) (map[string]interface{}, error) {
	if cached := c.Get(ctxJSONBody); cached != nil {
		m, _ := cached.(map[string]interface{})
		return m, nil
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return nil, err
	}

	m := map[string]interface{}{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &m); err != nil {
			return nil, err
		}
	}
	c.Set(ctxJSONBody, m)
	return m, nil
}
