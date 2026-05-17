package auth

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
)

// Resolved is the result of a successful token lookup, ready for the
// middleware to attach to the request context. Package-internal: the
// middleware in this package is the only caller.
type Resolved struct {
	User       models.User
	TokenID    int64
	Kind       string
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
}

// LookupRow is the joined user+token shape returned by
// [Repository.GetTokenByHash], used by the middleware to resolve a raw
// token to its owner without a second round trip.
type LookupRow struct {
	Token       models.Token
	UserID      int64
	Username    string
	UserCreated time.Time
}

// InsertTokenParams is the persistence-layer input for inserting a
// new auth_tokens row. Service builds this; repository writes it.
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

// Repository is the persistence surface for auth tokens. The service
// and middleware in this package depend on this interface; the
// concrete implementations in repository_sqlite.go and
// repository_postgres.go are the only things in the auth package that
// import sqlc-generated code.
type Repository interface {
	CreateToken(ctx context.Context, p InsertTokenParams) (models.Token, error)
	GetTokenByHash(ctx context.Context, tokenHash string) (LookupRow, error)
	DeleteTokenByID(ctx context.Context, userID, tokenID int64) (int64, error)
	DeleteTokenByHash(ctx context.Context, tokenHash string) error
}
