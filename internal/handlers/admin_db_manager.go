package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"cms-go/internal/dbmanager"

	"github.com/labstack/echo/v4"
)

// dbColumnJSON is the wire shape for dbmanager.ColumnInfo — a plain string
// type with json tags, since sql.NullString doesn't marshal cleanly on its
// own (Go 1.20 here, no generic sql.Null[T] JSON support).
type dbColumnJSON struct {
	Name         string  `json:"name"`
	UDTName      string  `json:"udt_name"`
	IsArray      bool    `json:"is_array"`
	Nullable     bool    `json:"nullable"`
	Default      *string `json:"default"`
	IsPrimaryKey bool    `json:"is_primary_key"`
}

func toDBColumnJSON(c dbmanager.ColumnInfo) dbColumnJSON {
	var def *string
	if c.Default.Valid {
		v := c.Default.String
		def = &v
	}
	return dbColumnJSON{
		Name: c.Name, UDTName: c.UDTName, IsArray: c.IsArray,
		Nullable: c.Nullable, Default: def, IsPrimaryKey: c.IsPrimaryKey,
	}
}

// GET /admin/db-manager — shell + the table list embedded as JSON.
func AdminDBManager(c echo.Context) error {
	ctx := c.Request().Context()
	tables, err := dbmanager.ListTables(ctx)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to list tables: "+err.Error())
	}

	b, err := json.Marshal(tables)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to encode tables")
	}

	data := map[string]interface{}{
		// template.JS (not a plain string) — renderWithLayout's blanket
		// string->template.HTML wrap does not stop JS-context escaping
		// inside a <script> tag; only template.JS does. See
		// AdminAPIBuilder for the same fix and full explanation.
		"TablesJSON": template.JS(b),
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/db_manager.html", data)
}

// GET /admin/db-manager/tables/:table/json — column list + PK info for the
// Structure tab and for the editor's has-primary-key gating.
func AdminDBManagerTableJSON(c echo.Context) error {
	ctx := c.Request().Context()
	table := c.Param("table")

	cols, err := dbmanager.TableColumns(ctx, table)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	out := make([]dbColumnJSON, len(cols))
	hasPK := false
	for i, col := range cols {
		out[i] = toDBColumnJSON(col)
		if col.IsPrimaryKey {
			hasPK = true
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"table":           table,
		"columns":         out,
		"has_primary_key": hasPK,
	})
}

// GET /admin/db-manager/tables/:table/rows?page=&page_size=&sort=&dir=&where=
func AdminDBManagerBrowseRows(c echo.Context) error {
	ctx := c.Request().Context()
	table := c.Param("table")

	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))

	result, err := dbmanager.BrowseRows(ctx, dbmanager.BrowseParams{
		Table:    table,
		SortCol:  c.QueryParam("sort"),
		SortDir:  c.QueryParam("dir"),
		Where:    c.QueryParam("where"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	cols := make([]dbColumnJSON, len(result.Columns))
	for i, col := range result.Columns {
		cols[i] = toDBColumnJSON(col)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"columns":   cols,
		"rows":      result.Rows,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
	})
}

// cellInputPayload is the JSON wire shape for dbmanager.CellInput.
type cellInputPayload struct {
	Value string `json:"value"`
	Null  bool   `json:"null"`
}

func toCellInputMap(m map[string]cellInputPayload) map[string]dbmanager.CellInput {
	out := make(map[string]dbmanager.CellInput, len(m))
	for k, v := range m {
		out[k] = dbmanager.CellInput{Value: v.Value, Null: v.Null}
	}
	return out
}

// POST /admin/db-manager/tables/:table/rows/new
func AdminDBManagerInsertRow(c echo.Context) error {
	ctx := c.Request().Context()
	table := c.Param("table")

	var req struct {
		Values map[string]cellInputPayload `json:"values"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	row, err := dbmanager.InsertRow(ctx, table, toCellInputMap(req.Values))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"row": row})
}

// POST /admin/db-manager/tables/:table/rows/edit
func AdminDBManagerUpdateRow(c echo.Context) error {
	ctx := c.Request().Context()
	table := c.Param("table")

	var req struct {
		Changes map[string]cellInputPayload `json:"changes"`
		PK      map[string]string           `json:"pk"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	row, err := dbmanager.UpdateRow(ctx, table, toCellInputMap(req.Changes), req.PK)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"row": row})
}

// POST /admin/db-manager/tables/:table/rows/delete
func AdminDBManagerDeleteRow(c echo.Context) error {
	ctx := c.Request().Context()
	table := c.Param("table")

	var req struct {
		PK map[string]string `json:"pk"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if err := dbmanager.DeleteRow(ctx, table, req.PK); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.NoContent(http.StatusOK)
}

// POST /admin/db-manager/sql — the live SQL console. Executes for real
// against the live database, no rollback wrapper (see dbmanager.RunSQL).
func AdminDBManagerRunSQL(c echo.Context) error {
	ctx := c.Request().Context()

	var req struct {
		SQL string `json:"sql"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if strings.TrimSpace(req.SQL) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "sql is required"})
	}

	rows, elapsed, err := dbmanager.RunSQL(ctx, req.SQL)
	resp := map[string]interface{}{
		"rows":       rows,
		"elapsed_ms": elapsed.Milliseconds(),
	}
	if err != nil {
		resp["error"] = err.Error()
	}
	return c.JSON(http.StatusOK, resp)
}
