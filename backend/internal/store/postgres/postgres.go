// Package postgres wires github.com/jackc/pgx/v5/stdlib (pure-Go) into
// database/sql for NextChapter. Postgres is the production option
// alongside the pure-Go SQLite default; pgx/v5's stdlib adapter keeps
// the build CGO-free, matching the rest of the project.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	// Register the pgx/v5 driver as "pgx" against the standard
	// database/sql package. The blank import is intentional: the rest
	// of this package only ever calls sql.Open("pgx", url).
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open opens a Postgres database via the pure-Go pgx/v5 stdlib driver.
// The URL is passed through as-is — pgx accepts the standard
// postgres:// (and postgresql://) scheme that libpq uses.
//
// The caller owns the returned handle and must Close it. The pool
// defaults are left untouched; Postgres handles concurrent writers
// natively, so the SQLite-style "max 1 open conn" cap doesn't apply.
func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		// Surface a close-time failure to the caller alongside the
		// ping error rather than swallowing it. errors.Join keeps both
		// in the returned error tree (errors.Is still works for either)
		// so we don't quietly mask a leak.
		pingErr := fmt.Errorf("postgres: ping: %w", err)
		if cerr := db.Close(); cerr != nil {
			return nil, errors.Join(pingErr, fmt.Errorf("postgres: close after ping: %w", cerr))
		}
		return nil, pingErr
	}
	return db, nil
}
