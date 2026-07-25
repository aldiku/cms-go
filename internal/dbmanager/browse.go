package dbmanager

import (
	"context"
	"fmt"
	"strings"

	"cms-go/internal/apiengine"
	"cms-go/internal/db"
)

type BrowseParams struct {
	Table    string
	SortCol  string
	SortDir  string // "asc" | "desc"
	Where    string // optional raw admin-authored boolean expression
	Page     int
	PageSize int
}

type BrowseResult struct {
	Columns  []ColumnInfo
	Rows     []map[string]interface{}
	Total    int64
	Page     int
	PageSize int
}

// BrowseRows lists a page of table's rows. Uses database/sql directly (via
// db.DB.DB()) with native $1,$2 placeholders — never gorm.Raw — because
// p.Where is untrusted admin-authored text that may itself contain a
// literal '?' (see package doc for why that matters).
func BrowseRows(ctx context.Context, p BrowseParams) (*BrowseResult, error) {
	cols, err := TableColumns(ctx, p.Table)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("table %q has no columns", p.Table)
	}
	byName := make(map[string]ColumnInfo, len(cols))
	for _, c := range cols {
		byName[c.Name] = c
	}

	if p.PageSize <= 0 {
		p.PageSize = 50
	}
	if p.Page <= 0 {
		p.Page = 1
	}

	sortCol := p.SortCol
	if sortCol == "" {
		for _, c := range cols {
			if c.IsPrimaryKey {
				sortCol = c.Name
				break
			}
		}
	} else if _, ok := byName[sortCol]; !ok {
		return nil, fmt.Errorf("unknown sort column %q", sortCol)
	}
	sortDir := "ASC"
	if strings.EqualFold(p.SortDir, "desc") {
		sortDir = "DESC"
	}

	selectCols := make([]string, len(cols))
	for i, c := range cols {
		ident := QuoteIdent(c.Name)
		if IsBytea(c) {
			selectCols[i] = fmt.Sprintf("encode(%s, 'hex') AS %s", ident, ident)
		} else {
			selectCols[i] = ident
		}
	}

	fromClause := QuoteIdent(p.Table)
	whereClause := ""
	if strings.TrimSpace(p.Where) != "" {
		whereClause = " WHERE " + p.Where
	}
	orderClause := ""
	if sortCol != "" {
		orderClause = fmt.Sprintf(" ORDER BY %s %s", QuoteIdent(sortCol), sortDir)
	}

	sqlDB, err := db.DB.DB()
	if err != nil {
		return nil, err
	}

	var total int64
	countQuery := "SELECT COUNT(*) FROM " + fromClause + whereClause
	if err := sqlDB.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, err
	}

	offset := (p.Page - 1) * p.PageSize
	// LIMIT/OFFSET are the only placeholders in this query, appended after
	// the (possibly $-free, admin-authored) WHERE text — no ambiguity with
	// anything inside it since database/sql sends the SQL verbatim.
	query := fmt.Sprintf("SELECT %s FROM %s%s%s LIMIT $1 OFFSET $2",
		strings.Join(selectCols, ", "), fromClause, whereClause, orderClause)

	sqlRows, err := sqlDB.QueryContext(ctx, query, p.PageSize, offset)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	rows, err := apiengine.ScanRows(sqlRows)
	if err != nil {
		return nil, err
	}

	return &BrowseResult{
		Columns:  cols,
		Rows:     rows,
		Total:    total,
		Page:     p.Page,
		PageSize: p.PageSize,
	}, nil
}
