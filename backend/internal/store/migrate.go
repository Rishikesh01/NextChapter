package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"

	"github.com/enable-it/nextchapter/backend/migrations"
)

// Migrate applies every pending migration in backend/migrations/ to db.
// It is safe to call on every startup: goose tracks applied versions in
// a metadata table and only runs new ones.
//
// The dialect is inferred from the databaseURL scheme to keep this
// package's surface narrow.
func Migrate(ctx context.Context, db *sql.DB, databaseURL string) error {
	dialect, err := dialectFor(databaseURL)
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(dialect, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("store: build goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("store: apply migrations: %w", err)
	}
	return nil
}

func dialectFor(databaseURL string) (goose.Dialect, error) {
	switch {
	case strings.HasPrefix(databaseURL, "sqlite://"):
		return goose.DialectSQLite3, nil
	case strings.HasPrefix(databaseURL, "postgres://"), strings.HasPrefix(databaseURL, "postgresql://"):
		return goose.DialectPostgres, nil
	default:
		return "", fmt.Errorf("store: cannot infer goose dialect from %q", databaseURL)
	}
}
