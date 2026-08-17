package auth

import (
	"database/sql"
	"time"
)

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
