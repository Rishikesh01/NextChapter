package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// AssertSessionGone fails t if an auth_tokens row exists for the hash
// of rawToken. Used by integration tests to verify /auth/logout's
// store-side effect without leaking the storage-hash format into the
// test — the same reason [hashToken] is package-private.
//
// SQLite-only: the integration suite's Postgres smoke test stays at
// the wire-shape level and doesn't drive this path.
func AssertSessionGone(t *testing.T, q *gen.Queries, rawToken string) {
	t.Helper()
	_, err := q.GetAuthTokenByHash(context.Background(), hashToken(rawToken))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("session row should be gone after logout (got err=%v)", err)
	}
}

// AssertSessionExists is the inverse of [AssertSessionGone]: the row
// for the hash of rawToken must be present.
func AssertSessionExists(t *testing.T, q *gen.Queries, rawToken string) {
	t.Helper()
	_, err := q.GetAuthTokenByHash(context.Background(), hashToken(rawToken))
	if err != nil {
		t.Fatalf("session row should exist (got err=%v)", err)
	}
}
