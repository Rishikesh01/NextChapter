package auth

import (
	"context"
	"time"

	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// Service owns mint/revoke flows over auth_tokens plus the read-side
// resolve/touch that the middleware in this package depends on. All SQL
// access goes through [Repository]; this file does not import the
// sqlc-generated package directly.
type Service struct {
	repo Repository
	now  func() time.Time
}

// User is the domain shape of an authenticated user, populated by the
// middleware and read by handlers via [UserFromContext]. It deliberately
// excludes [User.PasswordHash] — handlers must not be able to reach it
// off a request context. Login/register paths get the hash via the
// users.Service return type instead.
type User struct {
	ID        int64
	Username  string
	CreatedAt time.Time
}

// Resolved is the result of a successful token lookup, ready for the
// middleware to attach to the request context.
type Resolved struct {
	User       User
	TokenID    int64
	Kind       string
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
}

// Token is the domain shape for stored auth_tokens rows. It mirrors the
// columns the service and middleware actually use; the repository is the
// only thing in this package that knows about gen.AuthToken.
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

// LookupRow is the joined user+token shape returned by
// [Repository.GetTokenByHash], used by the middleware to resolve a raw
// token to its owner without a second round trip.
type LookupRow struct {
	Token        Token
	UserID       int64
	Username     string
	PasswordHash string
	UserCreated  time.Time
	UserUpdated  time.Time
}

// SessionToken is the result of [Service.CreateSession]: the raw token
// (already prefixed with constants.TokenPrefixSession) plus its DB row metadata.
type SessionToken struct {
	Raw   string
	Token Token
}

// APIToken is the result of [Service.CreateAPI].
type APIToken struct {
	Raw   string
	Token Token
}

// LoginParams is both the POST /auth/login JSON body and the input to
// [Service.Login]. No length bounds on the wire here — the credentials
// are matched against bcrypt; bounds-checking on input length would
// leak info.
type LoginParams struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// CreateTokenParams is both the POST /auth/tokens JSON body and the
// input to [Service.CreateAPI].
type CreateTokenParams struct {
	Label     string     `json:"label"                binding:"required,min=1,max=64"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// InsertTokenParams is the persistence-layer input for inserting a new
// auth_tokens row. Service builds this; repository writes it.
type InsertTokenParams struct {
	UserID     int64
	Kind       string
	TokenHash  string
	Label      string
	LabelValid bool
	CreatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
}

// TouchParams is the persistence-layer input for [Repository.Touch].
type TouchParams struct {
	ID         int64
	LastUsedAt *time.Time
	ExpiresAt  *time.Time // when non-nil, replaces expires_at; nil = leave alone.
}

// Repository is the persistence surface for auth tokens. The service and
// middleware in this package depend on this interface; the concrete
// implementation in [NewRepository] is the only thing in the auth
// package that imports the sqlc-generated code.
type Repository interface {
	CreateToken(ctx context.Context, p InsertTokenParams) (Token, error)
	GetTokenByHash(ctx context.Context, tokenHash string) (LookupRow, error)
	TouchToken(ctx context.Context, p TouchParams) error
	DeleteTokenByID(ctx context.Context, userID, tokenID int64) (int64, error)
	DeleteTokenByHash(ctx context.Context, tokenHash string) error
	ListAPITokens(ctx context.Context, userID int64) ([]Token, error)
	ListSessionTokens(ctx context.Context, userID int64) ([]Token, error)
}

// repository is the concrete sqlc-backed implementation of [Repository].
type repository struct {
	q *gen.Queries
}
