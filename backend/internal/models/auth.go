// Package models holds the cross-package wire / domain types shared
// between the HTTP handlers and the internal domain packages
// ([auth], [users], [series], [entries]). It exists so that
// [internal/httpapi/handlers] can depend on a single neutral package
// for request / response shapes and service interfaces, breaking the
// historical handlers -> auth -> handlers import cycle described in
// the project conventions doc.
//
// Rules of the road:
//   - Only types and tiny helpers (context attach/extract) live here.
//     No service implementations, no persistence code, no SQL.
//   - Persistence-layer structs (Insert*Params, Repository interfaces,
//     conversion shims) stay in their domain packages.
//   - The bidirectional invariant we keep in place: handlers only ever
//     import [internal/models]; the domain packages import [models]
//     for the wire shapes they accept/return.
package models

import (
	"context"
	"time"
)

// User is the public-facing authenticated user shape. It deliberately
// omits PasswordHash so handlers can never serialise it. The hash
// stays inside [internal/users]; only the users service sees it.
type User struct {
	ID        int64
	Username  string
	CreatedAt time.Time
}

// Token is the domain shape for stored auth_tokens rows. Mirrors the
// columns the auth service and middleware actually use.
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

// SessionToken is the result of [AuthService.CreateSession]: the raw
// token (already prefixed with constants.TokenPrefixSession) plus its
// DB row metadata.
type SessionToken struct {
	Raw   string
	Token Token
}

// APIToken is the result of [AuthService.CreateAPI]: the raw token
// (already prefixed with constants.TokenPrefixAPI) plus its DB row.
type APIToken struct {
	Raw   string
	Token Token
}

// Credentials is both the POST /auth/login JSON body and the input to
// [UsersService.Authenticate]. No length bounds on the wire — the
// credentials are matched against bcrypt and bounds-checking on input
// length would leak info.
type Credentials struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// NewToken is both the POST /auth/tokens JSON body and the input to
// [AuthService.CreateAPI].
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
