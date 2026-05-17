package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/enable-it/nextchapter/backend/internal/models"
	pg "github.com/enable-it/nextchapter/backend/internal/store/generated/pg"
)

// postgresRepo is the Postgres-backed implementation of [Repository].
// It wraps the pg-flavoured sqlc-generated *pg.Queries; the method
// shapes mirror sqliteRepo exactly so the service is engine-agnostic.
type postgresRepo struct {
	q *pg.Queries
}

func newPostgresRepository(db *sql.DB) *postgresRepo {
	return &postgresRepo{q: pg.New(db)}
}

func (r *postgresRepo) InsertUser(ctx context.Context, p InsertUserParams) (models.User, error) {
	u, err := r.q.CreateUser(ctx, pg.CreateUserParams{
		Username:     p.Username,
		PasswordHash: p.PasswordHash,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return models.User{}, models.ErrUsernameTaken
		}
		return models.User{}, fmt.Errorf("users: insert: %w", err)
	}
	return models.User{
		ID:        u.ID,
		Username:  u.Username,
		CreatedAt: u.CreatedAt,
	}, nil
}

func (r *postgresRepo) GetAuthRecordByUsername(ctx context.Context, username string) (AuthRecord, error) {
	u, err := r.q.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuthRecord{}, models.ErrUserNotFound
		}
		return AuthRecord{}, fmt.Errorf("users: get by username: %w", err)
	}
	return AuthRecord{
		ID:           u.ID,
		Username:     u.Username,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}, nil
}
