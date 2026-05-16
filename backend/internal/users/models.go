package users

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// authRecord is the package-internal shape that carries the password
// hash alongside the public user fields. The hash never escapes the
// users package: only [Service.Authenticate] reads it, and what
// crosses the package boundary is [models.User].
type authRecord struct {
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
// GetAuthRecordByUsername returns the package-internal authRecord
// because [Service.Authenticate] needs the hash to bcrypt-compare.
// Nothing outside this package can see authRecord.
type Repository interface {
	InsertUser(ctx context.Context, p InsertUserParams) (models.User, error)
	GetUserByID(ctx context.Context, id int64) (models.User, error)
	GetAuthRecordByUsername(ctx context.Context, username string) (authRecord, error)
	CountUsers(ctx context.Context) (int64, error)
}

// repository is the concrete sqlc-backed implementation of [Repository].
type repository struct {
	q *gen.Queries
}
