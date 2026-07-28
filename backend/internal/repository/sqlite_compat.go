package repository

import (
	"database/sql/driver"
	"sync"
	"time"

	"modernc.org/sqlite"
)

var sqliteCompatOnce sync.Once

// registerSQLiteCompatFunctions installs a small set of PostgreSQL-shaped helpers
// so existing repository SQL that uses NOW() keeps working under DIY/SQLite.
//
// This is intentionally minimal: full dialect parity is not a goal for DIY mode.
func registerSQLiteCompatFunctions() {
	sqliteCompatOnce.Do(func() {
		// NOW() → current UTC timestamp (same rough semantics as many PG deployments with UTC session).
		_ = sqlite.RegisterScalarFunction("now", 0, func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return time.Now().UTC().Format("2006-01-02 15:04:05.999999999"), nil
		})
		// Alias common casing used in raw SQL: NOW()
		_ = sqlite.RegisterScalarFunction("NOW", 0, func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return time.Now().UTC().Format("2006-01-02 15:04:05.999999999"), nil
		})
	})
}
