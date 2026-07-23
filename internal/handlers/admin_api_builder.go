package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"cms-go/internal/apiengine"
	"cms-go/internal/config"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

var allowedAPIMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
var allowedParamTypes = map[string]bool{"string": true, "integer": true, "float": true, "boolean": true, "date": true, "json": true}
var allowedParamSources = map[string]bool{"query": true, "body": true, "form": true, "path": true}

// apiEndpointSummary is the sidebar-list shape — cheap enough to embed in
// full for every endpoint on page load.
type apiEndpointSummary struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	GroupName string `json:"group_name"`
	Path      string `json:"path"`
	FullPath  string `json:"full_path"`
	Method    string `json:"method"`
	Status    uint8  `json:"status"`
}

func toAPIEndpointSummary(ep models.ApiEndpoint) apiEndpointSummary {
	return apiEndpointSummary{
		ID:        ep.ID,
		Name:      ep.Name,
		GroupName: ep.GroupName,
		Path:      ep.Path,
		FullPath:  ep.FullPath(),
		Method:    ep.Method,
		Status:    ep.Status,
	}
}

// apiEndpointPayload is the JSON body the builder's Save action sends —
// Parameters/Response are naturally nested structures edited as a whole, so
// this departs from the c.FormValue()-based convention used by Menus/Pages.
type apiEndpointPayload struct {
	Name       string                `json:"name"`
	GroupName  string                `json:"group_name"`
	Path       string                `json:"path"`
	Method     string                `json:"method"`
	Tags       string                `json:"tags"`
	Status     uint8                 `json:"status"`
	Parameters []models.ApiParameter `json:"parameters"`
	SQLText    string                `json:"sql_text"`
	Response   models.ResponseConfig `json:"response"`
}

// GET /admin/api-builder — the split-view shell. Embeds the sidebar summary
// list as JSON via json.Marshal's default (HTML-escaping) encoder into a
// <script type="application/json"> tag; renderWithLayout wraps this string
// in template.HTML so html/template doesn't re-escape the already-safe
// JSON. The editor pane is populated lazily per click via
// AdminAPIEndpointJSON.
func AdminAPIBuilder(c echo.Context) error {
	var endpoints []models.ApiEndpoint
	db.DB.Order("name asc").Find(&endpoints)

	summaries := make([]apiEndpointSummary, 0, len(endpoints))
	for _, ep := range endpoints {
		summaries = append(summaries, toAPIEndpointSummary(ep))
	}

	b, err := json.Marshal(summaries)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load endpoints")
	}

	data := map[string]interface{}{
		// template.JS (not a plain string) so renderWithLayout's blanket
		// string->template.HTML wrap doesn't touch it: template.HTML only
		// suppresses HTML-context escaping, but this value sits inside a
		// <script> tag where html/template applies JS-context escaping
		// regardless of that wrap — only template.JS marks it as
		// already-safe JS/JSON. json.Marshal's default encoder already
		// escapes '<', '>', '&' to \uXXXX sequences, so admin-authored free
		// text (Name/SQLText) containing "</script>" can't break out.
		"EndpointsJSON": template.JS(b),
		"APIBasePath":   config.APIBasePath(),
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/api_builder.html", data)
}

// GET /admin/api-builder/:id/json — full endpoint (decoded
// Parameters/Response) plus its revision history, for the editor pane.
func AdminAPIEndpointJSON(c echo.Context) error {
	id := c.Param("id")
	var ep models.ApiEndpoint
	if err := db.DB.First(&ep, id).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "endpoint not found"})
	}

	params, err := ep.Parameters()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to decode parameters"})
	}
	respCfg, err := ep.Response()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to decode response config"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":         ep.ID,
		"name":       ep.Name,
		"group_name": ep.GroupName,
		"path":       ep.Path,
		"method":     ep.Method,
		"tags":       ep.Tags,
		"status":     ep.Status,
		"parameters": params,
		"sql_text":   ep.SQLText,
		"response":   respCfg,
		// models.Revision has no json tags, so keys are Go-cased
		// (.ID/.CreatedAt/.UserName) — the Alpine template matches that.
		"revisions": loadRevisions("api_endpoint", ep.ID),
	})
}

// POST /admin/api-builder/new
func AdminCreateAPIEndpoint(c echo.Context) error {
	var payload apiEndpointPayload
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if err := validateAPIEndpointPayload(payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var ep models.ApiEndpoint
	if err := applyAPIEndpointPayload(&ep, payload); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if err := db.DB.Create(&ep).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create endpoint: " + err.Error()})
	}
	return c.JSON(http.StatusOK, toAPIEndpointSummary(ep))
}

// POST /admin/api-builder/:id/edit — the URL :id is the source of truth for
// which row to update; any id embedded in the JSON payload is ignored.
func AdminUpdateAPIEndpoint(c echo.Context) error {
	id := c.Param("id")
	var ep models.ApiEndpoint
	if err := db.DB.First(&ep, id).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "endpoint not found"})
	}
	before := ep

	var payload apiEndpointPayload
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if err := validateAPIEndpointPayload(payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := applyAPIEndpointPayload(&ep, payload); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if err := db.DB.Save(&ep).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update endpoint: " + err.Error()})
	}
	saveRevision(c, "api_endpoint", ep.ID, before)

	return c.JSON(http.StatusOK, toAPIEndpointSummary(ep))
}

// POST /admin/api-builder/:id/delete
func AdminDeleteAPIEndpoint(c echo.Context) error {
	id := c.Param("id")
	if err := db.DB.Delete(&models.ApiEndpoint{}, id).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete endpoint"})
	}
	return c.NoContent(http.StatusOK)
}

// POST /admin/api-builder/test — runs the CURRENT unsaved draft (never the
// stored row) against the real DB, wrapped in a transaction that is always
// rolled back so this is a true mock execution even for an
// INSERT/UPDATE/DELETE endpoint under active design. Returns raw debug JSON
// directly — intentionally bypasses ResponseConfig/BuildResponse, since the
// point of Test is to show ground truth before shaping the live output.
func AdminTestAPIEndpoint(c echo.Context) error {
	var req struct {
		SQLText    string                 `json:"sql_text"`
		Parameters []models.ApiParameter  `json:"parameters"`
		TestValues map[string]interface{} `json:"test_values"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	values, err := apiengine.ResolveTestValues(req.Parameters, req.TestValues)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	draft := models.ApiEndpoint{SQLText: req.SQLText}
	if err := draft.SetParameters(req.Parameters); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	tx := db.DB.Begin()
	defer tx.Rollback()

	rows, executedSQL, args, elapsed, execErr := apiengine.Execute(tx, draft, values)
	resp := map[string]interface{}{
		"executed_sql": executedSQL,
		"args":         args,
		"elapsed_ms":   elapsed.Milliseconds(),
		"rows":         rows,
	}
	if execErr != nil {
		resp["error"] = execErr.Error()
	}
	return c.JSON(http.StatusOK, resp)
}

func applyAPIEndpointPayload(ep *models.ApiEndpoint, p apiEndpointPayload) error {
	ep.Name = strings.TrimSpace(p.Name)
	ep.GroupName = strings.Trim(strings.TrimSpace(p.GroupName), "/")
	ep.Path = p.Path
	if !strings.HasPrefix(ep.Path, "/") {
		ep.Path = "/" + ep.Path
	}
	ep.Method = strings.ToUpper(strings.TrimSpace(p.Method))
	ep.Tags = p.Tags
	ep.Status = p.Status
	ep.SQLText = p.SQLText

	if err := ep.SetParameters(p.Parameters); err != nil {
		return err
	}
	return ep.SetResponse(p.Response)
}

// validateAPIEndpointPayload does plain Go checks only — no JSON-schema
// library is used anywhere in this repo, kept consistent here.
func validateAPIEndpointPayload(p apiEndpointPayload) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(p.SQLText) == "" {
		return fmt.Errorf("SQL text is required")
	}
	if !allowedAPIMethods[strings.ToUpper(strings.TrimSpace(p.Method))] {
		return fmt.Errorf("invalid method %q", p.Method)
	}
	path := p.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	pathParamNames := map[string]bool{}
	for _, seg := range strings.Split(strings.Trim(path, "/"), "/") {
		if strings.HasPrefix(seg, ":") {
			pathParamNames[strings.TrimPrefix(seg, ":")] = true
		}
	}

	seenNames := map[string]bool{}
	for _, param := range p.Parameters {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			return fmt.Errorf("parameter name cannot be empty")
		}
		if seenNames[name] {
			return fmt.Errorf("duplicate parameter name %q", name)
		}
		seenNames[name] = true
		if !allowedParamTypes[param.Type] {
			return fmt.Errorf("parameter %q has invalid type %q", name, param.Type)
		}
		if !allowedParamSources[param.Source] {
			return fmt.Errorf("parameter %q has invalid source %q", name, param.Source)
		}
		if param.Source == "path" && !pathParamNames[name] {
			return fmt.Errorf("parameter %q has source \"path\" but is not a :%s segment in path %q", name, name, p.Path)
		}
	}

	switch p.Response.Envelope {
	case "", "raw", "data", "custom":
	default:
		return fmt.Errorf("invalid response envelope %q", p.Response.Envelope)
	}
	switch p.Response.EmptyMode {
	case "", "empty_array", "null", "custom_message":
	default:
		return fmt.Errorf("invalid empty_mode %q", p.Response.EmptyMode)
	}
	switch p.Response.ErrorMode {
	case "", "generic", "detailed":
	default:
		return fmt.Errorf("invalid error_mode %q", p.Response.ErrorMode)
	}

	return nil
}
