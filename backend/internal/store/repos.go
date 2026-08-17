package store

import (
	"database/sql"
	"fmt"

	"github.com/enable-it/nextchapter/backend/internal/auth"
	"github.com/enable-it/nextchapter/backend/internal/entries"
	"github.com/enable-it/nextchapter/backend/internal/series"
	"github.com/enable-it/nextchapter/backend/internal/sites"
	"github.com/enable-it/nextchapter/backend/internal/users"
)

// Repos bundles the per-domain repository handles for a single dialect.
// Returned by [OpenRepos] so callers don't fan a per-domain dialect
// switch out across server wiring and integration helpers — adding a
// new domain becomes a one-site change here instead of one per consumer.
//
// Each field's interface type comes from its domain package. The
// interfaces have package-private methods, so store holds them as
// black-box values and the domain's NewService is the only thing that
// calls into them.
type Repos struct {
	Users   users.Repository
	Auth    auth.Repository
	Entries entries.Repository
	Series  series.Repository
	Sites   sites.Repository
}

// OpenRepos builds every domain repository for the given dialect with
// a single switch — the only place in the backend that maps dialect
// strings to per-engine concrete constructors. Callers pass the
// returned [Repos] straight into the per-domain NewService functions.
func OpenRepos(dialect string, db *sql.DB) (Repos, error) {
	switch dialect {
	case "sqlite3":
		return Repos{
			Users:   users.NewSQLiteRepository(db),
			Auth:    auth.NewSQLiteRepository(db),
			Entries: entries.NewSQLiteRepository(db),
			Series:  series.NewSQLiteRepository(db),
			Sites:   sites.NewSQLiteRepository(db),
		}, nil
	case "postgres":
		return Repos{
			Users:   users.NewPostgresRepository(db),
			Auth:    auth.NewPostgresRepository(db),
			Entries: entries.NewPostgresRepository(db),
			Series:  series.NewPostgresRepository(db),
			Sites:   sites.NewPostgresRepository(db),
		}, nil
	default:
		return Repos{}, fmt.Errorf("store: unknown dialect %q", dialect)
	}
}
