package series

import (
	"database/sql"
	"fmt"
	"time"
)

// NewRepository returns the engine-appropriate [Repository] for the
// given dialect. The dialect string mirrors what [store.DialectFor]
// returns ("sqlite3" or "postgres"). The repository keeps a handle on
// db so [SetSeriesTags] and the hand-rolled tag-filter queries can
// open transactions / run raw QueryContext respectively.
func NewRepository(dialect string, db *sql.DB) (Repository, error) {
	switch dialect {
	case "sqlite3":
		return newSQLiteRepository(db), nil
	case "postgres":
		return newPostgresRepository(db), nil
	default:
		return nil, fmt.Errorf("series: unknown dialect %q", dialect)
	}
}

// --- shared null / interface{} helpers -----------------------------------
//
// The rollup queries are typed as interface{} on the sqlc side because
// the correlated subqueries return NULL when a series has no entries.
// Driver behaviour varies — modernc.org/sqlite returns nil / int64 /
// float64 / string, pgx returns nil / float64 / time.Time — so we
// accept all of them and normalise to *float64 / *time.Time.

func nullInt64ToPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func intPtrToNullInt64(p *int) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*p), Valid: true}
}

// anyToFloatPtr handles the sqlc-generated `interface{}` columns from
// correlated subqueries. modernc.org/sqlite returns nil/int64/float64/
// string depending on affinity; pgx returns nil/float64. We accept any
// numeric form and fall through to nil for unrecognised shapes.
func anyToFloatPtr(v interface{}) *float64 {
	switch x := v.(type) {
	case nil:
		return nil
	case float64:
		return &x
	case float32:
		f := float64(x)
		return &f
	case int64:
		f := float64(x)
		return &f
	case int:
		f := float64(x)
		return &f
	default:
		return nil
	}
}

func anyToTimePtr(v interface{}) *time.Time {
	switch x := v.(type) {
	case nil:
		return nil
	case time.Time:
		if x.IsZero() {
			return nil
		}
		t := x
		return &t
	case string:
		if x == "" {
			return nil
		}
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.999999999Z07:00",
			"2006-01-02 15:04:05",
		} {
			if t, err := time.Parse(layout, x); err == nil {
				return &t
			}
		}
		return nil
	default:
		return nil
	}
}
