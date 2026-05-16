// Package store owns database connection management for NextChapter. It
// dispatches by URL scheme (sqlite, postgres) and returns a vanilla
// *sql.DB that the rest of the backend uses through the sqlc-generated
// Queries type.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/enable-it/nextchapter/backend/internal/store/postgres"
	"github.com/enable-it/nextchapter/backend/internal/store/sqlite"
)

// ErrUnsupportedScheme is returned when the database URL scheme is unknown
// or its driver is not yet implemented.
var ErrUnsupportedScheme = errors.New("store: unsupported database URL scheme")

// Open dispatches on the URL scheme and returns a configured *sql.DB.
// The caller owns the returned handle and must Close it.
func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	switch {
	case strings.HasPrefix(databaseURL, "sqlite://"):
		return sqlite.Open(ctx, databaseURL)
	case strings.HasPrefix(databaseURL, "postgres://"), strings.HasPrefix(databaseURL, "postgresql://"):
		return postgres.Open(ctx, databaseURL)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedScheme, databaseURL)
	}
}
