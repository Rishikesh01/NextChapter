// Package users owns the user-account domain. The per-engine repository
// implementations live in repository_sqlite.go / repository_postgres.go;
// the dialect-dispatch lives in [internal/store].OpenRepos, not here.
package users

import (
	"strings"
)

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
