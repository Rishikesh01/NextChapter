// Package migrations exposes the embedded SQL migration files for goose,
// split by dialect. Each engine has its own subdirectory:
//
//   - migrations/sqlite/  — SQLite-flavoured DDL (INTEGER PRIMARY KEY
//     AUTOINCREMENT, REAL, etc.)
//   - migrations/postgres/ — Postgres-flavoured DDL (BIGSERIAL, BIGINT,
//     DOUBLE PRECISION, etc.)
//
// The two trees carry the same numbered version sequence and identical
// semantics; the only differences are the engine-specific keywords and
// numeric types. [FS] picks the right subtree for a given goose dialect
// and returns an [fs.FS] rooted at that subtree so callers can pass it
// straight to [github.com/pressly/goose/v3].NewProvider.
package migrations

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed sqlite/*.sql
var sqliteFS embed.FS

//go:embed postgres/*.sql
var postgresFS embed.FS

// FS returns an [fs.FS] rooted at the per-dialect migrations subtree.
// Accepts the strings goose itself uses for [github.com/pressly/goose/v3].Dialect:
// "sqlite3" or "postgres". The returned FS is suitable for direct use
// as the third argument to [github.com/pressly/goose/v3].NewProvider.
func FS(dialect string) (fs.FS, error) {
	switch dialect {
	case "sqlite3":
		sub, err := fs.Sub(sqliteFS, "sqlite")
		if err != nil {
			return nil, fmt.Errorf("migrations: sub sqlite: %w", err)
		}
		return sub, nil
	case "postgres":
		sub, err := fs.Sub(postgresFS, "postgres")
		if err != nil {
			return nil, fmt.Errorf("migrations: sub postgres: %w", err)
		}
		return sub, nil
	default:
		return nil, fmt.Errorf("migrations: unknown dialect %q", dialect)
	}
}
