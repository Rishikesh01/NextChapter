package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"

	"github.com/enable-it/nextchapter/backend/migrations"
)

// Migrate applies every pending migration for the database's dialect
// to db. It is safe to call on every startup: goose tracks applied
// versions in a metadata table and only runs new ones.
//
// The dialect is inferred from the databaseURL scheme to keep this
// package's surface narrow. SQLite reads from migrations/sqlite/,
// Postgres from migrations/postgres/; both trees share the same
// version sequence.
func Migrate(ctx context.Context, db *sql.DB, databaseURL string) error {
	dialect, gooseDialect, err := dialectFor(databaseURL)
	if err != nil {
		return err
	}
	fsys, err := migrations.FS(dialect)
	if err != nil {
		return fmt.Errorf("store: locate migrations: %w", err)
	}
	provider, err := goose.NewProvider(gooseDialect, db, fsys)
	if err != nil {
		return fmt.Errorf("store: build goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("store: apply migrations: %w", err)
	}
	return nil
}

// DialectFor exposes the dialect-string mapping used by the migration
// and codegen layers. Returned as the plain string ("sqlite3" /
// "postgres") because callers outside this package (the server's
// dialect-aware repository factory) take it as a string.
func DialectFor(databaseURL string) (string, error) {
	d, _, err := dialectFor(databaseURL)
	return d, err
}

// dialectFor returns both the bare dialect string (for migrations.FS
// and the repository factories) and the goose-typed dialect for goose
// itself. The two are kept in lock-step here so a future engine only
// needs adding once.
func dialectFor(databaseURL string) (string, goose.Dialect, error) {
	switch {
	case strings.HasPrefix(databaseURL, "sqlite://"):
		return "sqlite3", goose.DialectSQLite3, nil
	case strings.HasPrefix(databaseURL, "postgres://"), strings.HasPrefix(databaseURL, "postgresql://"):
		return "postgres", goose.DialectPostgres, nil
	default:
		return "", "", fmt.Errorf("store: cannot infer dialect from %q", databaseURL)
	}
}
