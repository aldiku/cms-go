// Package dbmanager backs the admin "DB Manager" screen: a native Postgres
// table browser, row editor, and SQL console, scoped to the "public" schema.
//
// Identifier safety: Postgres has no bind-parameter support for table/column
// names, so every identifier that ends up in SQL text is first validated
// against a freshly-queried information_schema allow-list (never cached,
// never trusted from the client) and then quoted (QuoteIdent) as defense in
// depth. Values, in contrast, are always sent as real driver parameters.
//
// SQL execution: BrowseRows/InsertRow/UpdateRow/DeleteRow/RunSQL all bypass
// GORM's query builder (db.DB.Raw) and go through database/sql directly via
// db.DB.DB() — GORM's "?"->"$N" rewriter is a blind, quote-unaware byte scan
// (see clause/expression.go), so a literal '?' anywhere in admin-authored
// text (a string literal, a jsonb "?"/"?|"/"?&" operator) would silently
// steal a bind slot if mixed into the same gorm.Raw call as our own
// LIMIT/OFFSET args. database/sql sends SQL text verbatim via the extended
// query protocol — only real $N positions bind, so a stray '?' is inert.
// The only exception is the two single-$1-arg introspection queries below,
// which carry no untrusted SQL text and are safe via db.DB.Raw as-is.
package dbmanager

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"cms-go/internal/db"
)

// ColumnInfo describes one column of an introspected table.
type ColumnInfo struct {
	Name         string
	UDTName      string // e.g. "int4", "text", "_int4" (array), "my_enum"
	UDTSchema    string
	IsArray      bool
	Nullable     bool
	Default      sql.NullString
	IsPrimaryKey bool
	OrdinalPos   int
}

// ListTables returns every base table name in the public schema, sorted.
func ListTables(ctx context.Context) ([]string, error) {
	rows, err := db.DB.WithContext(ctx).Raw(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// tableAllowed reports whether table is a real public-schema base table,
// re-checked fresh (never cached) against the DB on every call.
func tableAllowed(ctx context.Context, table string) (bool, error) {
	tables, err := ListTables(ctx)
	if err != nil {
		return false, err
	}
	for _, t := range tables {
		if t == table {
			return true, nil
		}
	}
	return false, nil
}

// TableColumns returns table's columns (in ordinal order) with primary-key
// flags set, after validating table itself against the live allow-list.
func TableColumns(ctx context.Context, table string) ([]ColumnInfo, error) {
	ok, err := tableAllowed(ctx, table)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("table %q not found", table)
	}

	pk, err := pkColumns(ctx, table)
	if err != nil {
		return nil, err
	}

	rows, err := db.DB.WithContext(ctx).Raw(`
		SELECT column_name, data_type, udt_name, udt_schema,
		       is_nullable = 'YES' AS nullable, column_default, ordinal_position
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = ?
		ORDER BY ordinal_position`, table).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		var dataType string
		if err := rows.Scan(&c.Name, &dataType, &c.UDTName, &c.UDTSchema, &c.Nullable, &c.Default, &c.OrdinalPos); err != nil {
			return nil, err
		}
		c.IsArray = dataType == "ARRAY"
		c.IsPrimaryKey = pk[c.Name]
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func pkColumns(ctx context.Context, table string) (map[string]bool, error) {
	rows, err := db.DB.WithContext(ctx).Raw(`
		SELECT kcu.column_name FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON kcu.constraint_name = tc.constraint_name AND kcu.table_schema = tc.table_schema
		 AND kcu.table_name = tc.table_name
		WHERE tc.table_schema = 'public' AND tc.table_name = ?
		  AND tc.constraint_type = 'PRIMARY KEY'
		ORDER BY kcu.ordinal_position`, table).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pk := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		pk[name] = true
	}
	return pk, rows.Err()
}

// QuoteIdent wraps name as a safe Postgres double-quoted identifier. Defense
// in depth only — callers must validate the identifier against a live
// information_schema allow-list first; quoting alone doesn't restrict which
// real object is targeted.
func QuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// IsBytea reports whether col needs the encode/decode hex special-case
// instead of a "::type" cast (see package doc).
func IsBytea(col ColumnInfo) bool {
	return !col.IsArray && col.UDTName == "bytea"
}

// CastExpr returns the SQL cast target for col's type — "numeric", "int4[]"
// for arrays, or a quoted `"my_enum"` for custom/enum types. Never called
// for bytea columns (see IsBytea).
func CastExpr(col ColumnInfo) string {
	if col.IsArray {
		return strings.TrimPrefix(col.UDTName, "_") + "[]"
	}
	if col.UDTSchema != "" && col.UDTSchema != "pg_catalog" {
		return QuoteIdent(col.UDTSchema) + "." + QuoteIdent(col.UDTName)
	}
	return col.UDTName
}
