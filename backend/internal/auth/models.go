package auth

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
)

// resolved is the result of a successful token lookup, ready for the
// middleware to attach to the request context. Package-internal: the
// middleware in this package is the only caller.
type resolved struct {
	User       models.User
	TokenID    int64
	Kind       string
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
}

// tokenWithUser is the joined user+token shape returned by
// [repository.getTokenByHash], used by the middleware to resolve a raw
// token to its owner without a second round trip.
type tokenWithUser struct {
	Token       models.Token
	UserID      int64
	Username    string
	UserCreated time.Time
}

// insertTokenParams is the persistence-layer input for inserting a
// new auth_tokens row. Service builds this; repository writes it.
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

// repository is the persistence surface for auth tokens. The service
// and middleware in this package depend on this interface; the
// concrete implementations in repository_sqlite.go and
// repository_postgres.go are the only things in the auth package that
// import sqlc-generated code.
type repository interface {
	createToken(ctx context.Context, p insertTokenParams) (models.Token, error)
	getTokenByHash(ctx context.Context, tokenHash string) (tokenWithUser, error)
	deleteTokenByID(ctx context.Context, userID, tokenID int64) (int64, error)
	deleteTokenByHash(ctx context.Context, tokenHash string) error
}
