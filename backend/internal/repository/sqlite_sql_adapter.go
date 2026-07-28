package repository

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"
)

// postgresPlaceholderRe matches $1, $2, ... used by lib/pq-style queries.
// It intentionally ignores $$ dollar-quoting openers by requiring a digit.
var postgresPlaceholderRe = regexp.MustCompile(`\$(\d+)`)

// rewritePostgresPlaceholders converts $1..$N placeholders to SQLite "?".
// Arguments are reordered to match the first-seen $n order if needed; most
// queries already use sequential $1..$N so args are left as-is when dense.
func rewritePostgresPlaceholders(query string, args []any) (string, []any) {
	if !strings.Contains(query, "$") {
		return query, args
	}

	// Collect unique indices in order of appearance.
	matches := postgresPlaceholderRe.FindAllStringSubmatchIndex(query, -1)
	if len(matches) == 0 {
		return query, args
	}

	maxIdx := 0
	seenOrder := make([]int, 0, len(matches))
	seenSet := make(map[int]struct{}, len(matches))
	for _, m := range matches {
		n, err := strconv.Atoi(query[m[2]:m[3]])
		if err != nil || n <= 0 {
			continue
		}
		if n > maxIdx {
			maxIdx = n
		}
		if _, ok := seenSet[n]; !ok {
			seenSet[n] = struct{}{}
			seenOrder = append(seenOrder, n)
		}
	}

	// Dense sequential $1..$N with same length as args: simple replace.
	sequential := len(seenOrder) == maxIdx && maxIdx == len(args)
	if sequential {
		for i, n := range seenOrder {
			if n != i+1 {
				sequential = false
				break
			}
		}
	}

	out := postgresPlaceholderRe.ReplaceAllString(query, "?")
	if sequential || len(args) == 0 {
		return out, args
	}

	// Sparse / non-sequential: rebuild args in appearance order.
	// $n is 1-based into the original args slice.
	rebuilt := make([]any, 0, len(matches))
	for _, m := range matches {
		n, err := strconv.Atoi(query[m[2]:m[3]])
		if err != nil || n <= 0 || n > len(args) {
			rebuilt = append(rebuilt, nil)
			continue
		}
		rebuilt = append(rebuilt, args[n-1])
	}
	return out, rebuilt
}

// sqliteDB adapts *sql.DB so existing $n SQL works on SQLite.
type sqliteDB struct {
	db *sql.DB
}

func newSQLiteDB(db *sql.DB) *sqliteDB {
	return &sqliteDB{db: db}
}

func (s *sqliteDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	q, a := rewritePostgresPlaceholders(query, args)
	return s.db.QueryContext(ctx, q, a...)
}

func (s *sqliteDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	q, a := rewritePostgresPlaceholders(query, args)
	return s.db.ExecContext(ctx, q, a...)
}

// Unwrap returns the underlying *sql.DB (for pool settings / ping / close via ent).
func (s *sqliteDB) Unwrap() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

// adaptSQLExecutor rewrites lib/pq $n placeholders for SQLite when needed.
// Pass-through for postgres and for already-adapted executors.
func adaptSQLExecutor(sqlq sqlExecutor) sqlExecutor {
	if sqlq == nil {
		return nil
	}
	if !IsSQLiteDialect() {
		return sqlq
	}
	if _, ok := sqlq.(*sqliteDB); ok {
		return sqlq
	}
	if db, ok := sqlq.(*sql.DB); ok {
		return newSQLiteDB(db)
	}
	// *sql.Tx and other executors: wrap with a thin adapter.
	return &sqliteExecutor{inner: sqlq}
}

// sqliteExecutor rewrites placeholders for any sqlExecutor (including *sql.Tx).
type sqliteExecutor struct {
	inner sqlExecutor
}

func (s *sqliteExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	q, a := rewritePostgresPlaceholders(query, args)
	return s.inner.QueryContext(ctx, q, a...)
}

func (s *sqliteExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	q, a := rewritePostgresPlaceholders(query, args)
	return s.inner.ExecContext(ctx, q, a...)
}

// batchImageSQL is *sql.DB plus QueryRowContext with $n rewrite for SQLite.
type batchImageSQL struct {
	db *sql.DB
}

func adaptBatchImageSQL(db *sql.DB) *batchImageSQL {
	return &batchImageSQL{db: db}
}

func (b *batchImageSQL) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if !IsSQLiteDialect() {
		return b.db.QueryContext(ctx, query, args...)
	}
	q, a := rewritePostgresPlaceholders(query, args)
	return b.db.QueryContext(ctx, q, a...)
}

func (b *batchImageSQL) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if !IsSQLiteDialect() {
		return b.db.ExecContext(ctx, query, args...)
	}
	q, a := rewritePostgresPlaceholders(query, args)
	return b.db.ExecContext(ctx, q, a...)
}

func (b *batchImageSQL) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if !IsSQLiteDialect() {
		return b.db.QueryRowContext(ctx, query, args...)
	}
	q, a := rewritePostgresPlaceholders(query, args)
	return b.db.QueryRowContext(ctx, q, a...)
}
