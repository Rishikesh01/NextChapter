// Package postgres is a placeholder for the future Postgres driver
// wiring. ADR-0002 declares Postgres a production option but the
// bootstrap milestone only ships SQLite; this file exists so the
// store-level driver dispatch in [store.Open] can compile against a
// real symbol and return a clear error to operators who try
// postgres:// today.
package postgres

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNotImplemented is returned by Open while Postgres support is on the
// roadmap but not yet wired up.
var ErrNotImplemented = errors.New("postgres: driver not yet implemented in this build")

// Open is a stub that always returns [ErrNotImplemented].
func Open(_ context.Context, _ string) (*sql.DB, error) {
	return nil, ErrNotImplemented
}
