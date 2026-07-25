package dbmanager

import (
	"context"
	"fmt"
	"strings"

	"cms-go/internal/apiengine"
	"cms-go/internal/db"
)

// CellInput is one field's submitted value. Null is checked independently
// of Value so empty-string vs. NULL is an explicit choice, never inferred.
type CellInput struct {
	Value string
	Null  bool
}

// buildAssignment returns the SQL fragment for binding one column's value
// at placeholder position n ("$N::type" or, for bytea, "decode($N,'hex')"),
// and the arg to bind there (nil when in.Null).
func buildAssignment(col ColumnInfo, in CellInput, n int) (expr string, arg interface{}) {
	if in.Null {
		arg = nil
	} else {
		arg = in.Value
	}
	if IsBytea(col) {
		return fmt.Sprintf("decode($%d,'hex')", n), arg
	}
	return fmt.Sprintf("$%d::%s", n, CastExpr(col)), arg
}

// validateColumns checks every key of values against table's real columns
// (freshly introspected), returning the column map for reuse by callers.
func validateColumns(ctx context.Context, table string, values map[string]CellInput) (map[string]ColumnInfo, error) {
	cols, err := TableColumns(ctx, table)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]ColumnInfo, len(cols))
	for _, c := range cols {
		byName[c.Name] = c
	}
	for name := range values {
		if _, ok := byName[name]; !ok {
			return nil, fmt.Errorf("unknown column %q", name)
		}
	}
	return byName, nil
}

func runReturningOne(ctx context.Context, query string, args []interface{}) (map[string]interface{}, error) {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return nil, err
	}
	rows, err := sqlDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scanned, err := apiengine.ScanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(scanned) == 0 {
		return nil, nil
	}
	return scanned[0], nil
}

// InsertRow inserts one row built from values (only the provided columns
// are included — omitted columns fall back to the table's own DEFAULT).
func InsertRow(ctx context.Context, table string, values map[string]CellInput) (map[string]interface{}, error) {
	byName, err := validateColumns(ctx, table, values)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("no values provided")
	}

	cols := make([]string, 0, len(values))
	placeholders := make([]string, 0, len(values))
	args := make([]interface{}, 0, len(values))
	n := 1
	for name, in := range values {
		col := byName[name]
		cols = append(cols, QuoteIdent(name))
		expr, arg := buildAssignment(col, in, n)
		placeholders = append(placeholders, expr)
		args = append(args, arg)
		n++
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *",
		QuoteIdent(table), strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	return runReturningOne(ctx, query, args)
}

// UpdateRow applies changes to the single row matched by pk (primary-key
// column -> value). Serves both single-cell inline edit (one-key changes
// map) and a whole-row edit form without duplicating WHERE-building logic.
func UpdateRow(ctx context.Context, table string, changes map[string]CellInput, pk map[string]string) (map[string]interface{}, error) {
	if len(pk) == 0 {
		return nil, fmt.Errorf("table %q has no primary key — editing is disabled", table)
	}
	if len(changes) == 0 {
		return nil, fmt.Errorf("no changes provided")
	}

	cols, err := TableColumns(ctx, table)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]ColumnInfo, len(cols))
	for _, c := range cols {
		byName[c.Name] = c
	}
	for name := range changes {
		if _, ok := byName[name]; !ok {
			return nil, fmt.Errorf("unknown column %q", name)
		}
	}
	for name := range pk {
		if _, ok := byName[name]; !ok {
			return nil, fmt.Errorf("unknown primary key column %q", name)
		}
	}

	setClauses := make([]string, 0, len(changes))
	args := make([]interface{}, 0, len(changes)+len(pk))
	n := 1
	for name, in := range changes {
		expr, arg := buildAssignment(byName[name], in, n)
		setClauses = append(setClauses, fmt.Sprintf("%s = %s", QuoteIdent(name), expr))
		args = append(args, arg)
		n++
	}

	whereClauses := make([]string, 0, len(pk))
	for name, val := range pk {
		expr, arg := buildAssignment(byName[name], CellInput{Value: val}, n)
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", QuoteIdent(name), expr))
		args = append(args, arg)
		n++
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s RETURNING *",
		QuoteIdent(table), strings.Join(setClauses, ", "), strings.Join(whereClauses, " AND "))
	return runReturningOne(ctx, query, args)
}

// DeleteRow deletes the single row matched by pk.
func DeleteRow(ctx context.Context, table string, pk map[string]string) error {
	if len(pk) == 0 {
		return fmt.Errorf("table %q has no primary key — deleting is disabled", table)
	}
	cols, err := TableColumns(ctx, table)
	if err != nil {
		return err
	}
	byName := make(map[string]ColumnInfo, len(cols))
	for _, c := range cols {
		byName[c.Name] = c
	}
	for name := range pk {
		if _, ok := byName[name]; !ok {
			return fmt.Errorf("unknown primary key column %q", name)
		}
	}

	whereClauses := make([]string, 0, len(pk))
	args := make([]interface{}, 0, len(pk))
	n := 1
	for name, val := range pk {
		col := byName[name]
		expr, arg := buildAssignment(col, CellInput{Value: val}, n)
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", QuoteIdent(name), expr))
		args = append(args, arg)
		n++
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s", QuoteIdent(table), strings.Join(whereClauses, " AND "))

	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	_, err = sqlDB.ExecContext(ctx, query, args...)
	return err
}
