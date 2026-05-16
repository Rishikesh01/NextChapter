package users

import (
	"context"
	"time"

	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// Service exposes the user domain to handlers.
type Service struct {
	repo Repository
	now  func() time.Time
}

// User is the domain shape of a user row. It includes the password
// hash, which the auth service needs for /auth/login; handlers must
// never write PasswordHash into a response.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RegisterParams is both the POST /auth/register JSON body and the
// input to [Service.Create]. Bounds mirror the openapi spec: username
// 1-64, password 8-256.
type RegisterParams struct {
	Username string `json:"username" binding:"required,min=1,max=64"`
	Password string `json:"password" binding:"required,min=8,max=256"`
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
type Repository interface {
	InsertUser(ctx context.Context, p InsertUserParams) (User, error)
	GetUserByID(ctx context.Context, id int64) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	CountUsers(ctx context.Context) (int64, error)
}

// repository is the concrete sqlc-backed implementation of [Repository].
type repository struct {
	q *gen.Queries
}
