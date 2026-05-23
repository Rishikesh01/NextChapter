package auth

import (
	"database/sql"
	"fmt"
	"time"
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
		return nil, fmt.Errorf("auth: unknown dialect %q", dialect)
	}
}

// --- shared null-time helpers --------------------------------------------
//
// Both engine variants project sql.NullTime <-> *time.Time the same way;
// keeping the helpers here lets the per-engine files stay focused on the
// generated-type translation.

func nullTimeToPtr(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	v := n.Time
	return &v
}

func timePtrToNullTime(p *time.Time) sql.NullTime {
	if p == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *p, Valid: true}
}
