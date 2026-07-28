package repository

import (
	"strings"
	"sync/atomic"
)

// SQL dialect identifiers used by repository raw SQL branches.
const (
	DialectPostgres = "postgres"
	DialectSQLite   = "sqlite"
)

// activeDialect holds the process-wide SQL dialect selected at InitEnt time.
// Default is postgres for backward compatibility with standard deployments.
var activeDialect atomic.Value // string

func init() {
	activeDialect.Store(DialectPostgres)
}

// SetActiveDialect records which SQL dialect the process is using.
// Called once from InitEnt / OpenSQLiteForSetup.
func SetActiveDialect(dialect string) {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case DialectSQLite, "sqlite3", "modernc":
		activeDialect.Store(DialectSQLite)
	default:
		activeDialect.Store(DialectPostgres)
	}
}

// ActiveDialect returns the current process dialect (postgres|sqlite).
func ActiveDialect() string {
	if v := activeDialect.Load(); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return DialectPostgres
}

// IsSQLiteDialect reports whether the process is running against SQLite.
func IsSQLiteDialect() bool {
	return ActiveDialect() == DialectSQLite
}

// sqlNowExpr returns a SQL expression for the current timestamp in the active dialect.
func sqlNowExpr() string {
	if IsSQLiteDialect() {
		return "datetime('now')"
	}
	return "NOW()"
}
