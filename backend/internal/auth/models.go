package auth

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
)

// Resolved is the result of a successful token lookup, ready for the
// auth middleware in [internal/httpapi/middleware] to attach to the
// request context — that middleware is the only caller. Exported because
// it is the return type of [Service.Resolve], which crosses the package
// boundary into httpapi/middleware.
type Resolved struct {
	User       models.User
	TokenID    int64
	Kind       string
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
}

// tokenWithUser is the joined user+token shape returned by
// [Repository.getTokenByHash], used by the middleware to resolve a raw
// token to its owner without a second round trip.
type tokenWithUser struct {
	Token       models.Token
	UserID      int64
	Username    string
	UserCreated time.Time
}

// insertTokenParams is the persistence-layer input for inserting a
// new auth_tokens row. Service builds this; Repository writes it.
type insertTokenParams struct {
	UserID     int64
	Kind       string
	TokenHash  string
	Label      string
	LabelValid bool
	CreatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
}

// Repository is the persistence surface for auth tokens. The service
// and middleware in this package depend on this interface; the
// concrete implementations in Repository_sqlite.go and
// Repository_postgres.go are the only things in the auth package that
// import sqlc-generated code.
type Repository interface {
	createToken(ctx context.Context, p insertTokenParams) (models.Token, error)
	getTokenByHash(ctx context.Context, tokenHash string) (tokenWithUser, error)
	deleteTokenByID(ctx context.Context, userID, tokenID int64) (int64, error)
	deleteTokenByHash(ctx context.Context, tokenHash string) error
}
