package handlers

import (
	"net/http"
	"strings"

	"cms-go/internal/apiengine"
	"cms-go/internal/config"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// PublicAPIDispatch is the single physical route backing every admin-
// defined API Builder endpoint. It strips config.APIBasePath() off the
// request path, looks up the matching active ApiEndpoint by method +
// remaining path, enforces the "auth" tag (X-API-Key header must equal
// config.APIKey() — an unset API_KEY fails closed, rejecting everything
// rather than silently going public), resolves param values, executes for
// real (no rollback wrapper — this is genuine traffic), and writes the
// ResponseConfig-shaped JSON. Executed SQL text is never available in this
// function's inputs at all, so it can never leak into a live response.
func PublicAPIDispatch(c echo.Context) error {
	path := strings.TrimPrefix(c.Request().URL.Path, config.APIBasePath())
	if path == "" {
		path = "/"
	}

	ep, pathParams, ok := apiengine.FindEndpoint(db.DB, c.Request().Method, path)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}

	if ep.HasTag("auth") {
		key := config.APIKey()
		if key == "" || c.Request().Header.Get("X-API-Key") != key {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}
	}

	params, err := ep.Parameters()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	var bodyJSON map[string]interface{}
	for _, p := range params {
		if p.Source == "body" {
			bodyJSON, err = apiengine.ReadJSONBody(c)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			}
			break
		}
	}

	values, err := apiengine.ResolveParamValues(c, params, bodyJSON, pathParams)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	rows, _, _, elapsed, execErr := apiengine.Execute(db.DB, ep, values)

	respCfg, err := ep.Response()
	if err != nil {
		respCfg = models.ResponseConfig{}
	}

	status, body := apiengine.BuildResponse(respCfg, rows, execErr, elapsed.Milliseconds())
	return c.JSON(status, body)
}
