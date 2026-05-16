// Package models holds the cross-package wire / domain value types
// shared between the HTTP handlers and the internal domain packages
// ([auth], [users], [series], [entries]). It exists so that
// [internal/httpapi/handlers] depends on a single neutral package for
// request / response shapes plus the four domain packages strictly
// for their service interfaces — never for the domain value types
// themselves.
//
// Rules of the road:
//   - Only value types and tiny helpers (context attach/extract) live
//     here. No service implementations, no persistence code, no SQL.
//   - Service interfaces live in their domain package
//     (auth.AuthService, users.UsersService, series.SeriesService,
//     entries.EntriesService), not here.
//   - Persistence-layer structs (Insert*Params, Repository interfaces,
//     conversion shims) stay in their domain packages.
//   - Domain models ARE the wire shape: every type returned by a
//     service carries json tags and is written directly via
//     c.JSON(status, model). No *Response wrappers in handlers, no
//     xToJSON mappers. Internal-only fields (UserID, TokenHash,
//     PasswordHash, etc.) are tagged `json:"-"` so they never leak.
package models

import (
	"context"
	"time"
)

// User is the public-facing authenticated user shape and the wire
// shape for /auth/me, /auth/login, /auth/register. It deliberately
// omits PasswordHash so handlers can never serialise it; the hash
// stays inside [internal/users] and only the users service sees it.
type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// Token is the domain shape for stored auth_tokens rows. Mirrors the
// columns the auth service and middleware actually use. Token itself
// is never serialised to the wire directly (it carries TokenHash and
// the wire never returns the hash); /auth/tokens responses use
// [APIToken] which carries only the wire-safe fields.
type Token struct {
	ID         int64
	UserID     int64
	Kind       string
	TokenHash  string
	Label      string
	LabelValid bool
	CreatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
}

// SessionToken is the result of the auth service's CreateSession
// method: the raw token (already prefixed with
// constants.TokenPrefixSession) plus its DB row metadata. Internal
// only — handlers consume the Raw field to set the cookie and never
// serialise this struct to the wire. Every field is `json:"-"` so an
// accidental c.JSON would emit `{}`.
type SessionToken struct {
	Raw   string `json:"-"`
	Token Token  `json:"-"`
}

// APIToken is the POST /auth/tokens response and the result of the
// auth service's CreateAPIToken method. The wire shape carries the
// stored row fields plus the raw plaintext token (returned exactly
// once at creation time). All non-wire fields on the underlying
// auth_tokens row (UserID, Kind, TokenHash, LabelValid) are kept off
// this type entirely.
type APIToken struct {
	ID         int64      `json:"id"`
	Label      string     `json:"label"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	// Raw is the plaintext token, surfaced exactly once on
	// POST /auth/tokens. The server stores only the hash; subsequent
	// reads return APIToken values with Raw == "" (omitempty drops the
	// key entirely from the wire payload).
	Raw string `json:"token,omitempty"`
}

// Credentials is both the POST /auth/login JSON body and the input to
// the auth service's Authenticate method. No length bounds on the
// wire — the credentials are matched against bcrypt and
// bounds-checking on input length would leak info.
type Credentials struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// NewToken is both the POST /auth/tokens JSON body and the input to
// the auth service's CreateAPIToken method.
type NewToken struct {
	Label     string     `json:"label"                binding:"required,min=1,max=64"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// userCtxKey is the unexported context-key type used by [WithUser] and
// [UserFromContext]. Unexported so other packages can't collide.
type userCtxKey struct{}

// WithUser returns a new context that carries u. The auth middleware
// calls this; handlers read the user back via [UserFromContext].
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userCtxKey{}, u)
}

// UserFromContext returns the authenticated user attached to ctx by
// the auth middleware. The bool reports presence; handlers behind the
// auth middleware can assume true.
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userCtxKey{}).(User)
	return u, ok
}
