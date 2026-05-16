package users

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// AuthRecord is the package-boundary shape that carries the password
// hash alongside the public user fields. It is consumed exclusively by
// [internal/auth].Service.Authenticate: the auth service reads
// PasswordHash to bcrypt-compare and returns [models.User] to its
// caller, so the hash never crosses into the handler layer.
//
// Exporting this type is the cost of moving the credential-verification
// flow into the auth package; the alternative (returning a tuple of
// (hash, user, error)) is uglier on the call site for no real
// containment win.
type AuthRecord struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// InsertUserParams is the persistence input for [Repository.InsertUser].
type InsertUserParams struct {
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Repository is the persistence surface for the users domain. The
// service in this package depends on this interface; the concrete
// implementation in [NewRepository] is the only thing in the package
// that imports the sqlc-generated code.
//
// InsertUser returns the public-facing [models.User] (no hash);
// GetAuthRecordByUsername returns [AuthRecord] because the auth
// package's Authenticate needs the hash to bcrypt-compare.
type Repository interface {
	InsertUser(ctx context.Context, p InsertUserParams) (models.User, error)
	GetUserByID(ctx context.Context, id int64) (models.User, error)
	GetAuthRecordByUsername(ctx context.Context, username string) (AuthRecord, error)
	CountUsers(ctx context.Context) (int64, error)
}

// repository is the concrete sqlc-backed implementation of [Repository].
type repository struct {
	q *gen.Queries
}
