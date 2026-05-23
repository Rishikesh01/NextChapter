package entries

import (
	"database/sql"
	"fmt"
)

// NewRepository returns the engine-appropriate repository for the
// given dialect. The dialect string mirrors what [store.DialectFor]
// returns ("sqlite3" or "postgres").
func NewRepository(dialect string, db *sql.DB) (repository, error) {
	switch dialect {
	case "sqlite3":
		return newSQLiteRepository(db), nil
	case "postgres":
		return newPostgresRepository(db), nil
	default:
		return nil, fmt.Errorf("entries: unknown dialect %q", dialect)
	}
}
