// Package users owns the user-account domain. The repository in this
// file is the dispatcher: it inspects the dialect string ("sqlite3" or
// "postgres") and returns the engine-specific implementation. The
// concrete impls live in repository_sqlite.go and repository_postgres.go.
package users

import (
	"database/sql"
	"fmt"
	"strings"
)

// NewRepository returns the engine-appropriate [Repository] for the
// given dialect. The dialect string mirrors what [store.DialectFor]
// returns ("sqlite3" or "postgres").
func NewRepository(dialect string, db *sql.DB) (Repository, error) {
	switch dialect {
	case "sqlite3":
		return newSQLiteRepository(db), nil
	case "postgres":
		return newPostgresRepository(db), nil
	default:
		return nil, fmt.Errorf("users: unknown dialect %q", dialect)
	}
}

// isUniqueViolation detects the engine-agnostic "unique constraint
// failed" signal. Both modernc.org/sqlite and pgx surface a textual
// hint; matching on the substring is the lowest-common-denominator
// approach and avoids depending on either driver's internal error
// types.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "UNIQUE constraint failed"),
		strings.Contains(s, "constraint failed: UNIQUE"),
		// pgx surfaces this as "duplicate key value violates unique
		// constraint" — match the prefix so any constraint name is OK.
		strings.Contains(s, "duplicate key value violates unique constraint"),
		strings.Contains(s, "SQLSTATE 23505"):
		return true
	}
	return false
}
