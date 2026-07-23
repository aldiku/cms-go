package apiengine

import (
	"time"

	"cms-go/internal/models"

	"gorm.io/gorm"
)

// Execute runs ep's SQL against gdb with resolved param values bound as
// real positional args (never string-concatenated), using
// gdb.Raw(sql, args...).Rows() uniformly for SELECT and DML alike — an
// INSERT/UPDATE/DELETE without a RETURNING clause simply scans zero rows
// (no affected-row-count tracking in v1). executedSQL/args are debug-only
// return values: callers must never surface them in a live public response
// (see BuildResponse, which doesn't even accept them as a parameter).
func Execute(gdb *gorm.DB, ep models.ApiEndpoint, values map[string]interface{}) (rows []map[string]interface{}, executedSQL string, args []interface{}, elapsed time.Duration, err error) {
	params, err := ep.Parameters()
	if err != nil {
		return nil, "", nil, 0, err
	}

	rewritten, occurrences := ParsePlaceholders(ep.SQLText)
	args, err = BindArgs(occurrences, params, values)
	if err != nil {
		return nil, rewritten, nil, 0, err
	}

	start := time.Now()
	sqlRows, err := gdb.Raw(rewritten, args...).Rows()
	if err != nil {
		return nil, rewritten, args, time.Since(start), err
	}
	defer sqlRows.Close()

	rows, err = ScanRows(sqlRows)
	elapsed = time.Since(start)
	return rows, rewritten, args, elapsed, err
}
