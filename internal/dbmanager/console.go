package dbmanager

import (
	"context"
	"time"

	"cms-go/internal/apiengine"
	"cms-go/internal/db"
)

// RunSQL executes sqlText verbatim against the live database — no rollback
// wrapper, unlike the API Builder's Test tool: this is the live console,
// same expectation as pgAdmin/DBeaver's query tool. Called with zero bind
// args, so it's inherently unaffected by the '?'-scan issue described in
// the package doc. No affected-row-count tracking (same accepted
// limitation as apiengine.Execute). One statement per call — pgx's
// extended query protocol doesn't support ';'-separated multi-statements in
// a single Query call.
func RunSQL(ctx context.Context, sqlText string) (rows []map[string]interface{}, elapsed time.Duration, err error) {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return nil, 0, err
	}

	start := time.Now()
	sqlRows, err := sqlDB.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, time.Since(start), err
	}
	defer sqlRows.Close()

	rows, err = apiengine.ScanRows(sqlRows)
	elapsed = time.Since(start)
	return rows, elapsed, err
}
