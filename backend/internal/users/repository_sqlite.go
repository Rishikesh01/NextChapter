package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// sqliteRepo is the SQLite-backed implementation of [Repository].
// It wraps the sqlc-generated *gen.Queries; the integration tests
// also wrap the same Queries handle directly to assert store-state.
type sqliteRepo struct {
	q *gen.Queries
}

func NewSQLiteRepository(db *sql.DB) *sqliteRepo {
	return &sqliteRepo{q: gen.New(db)}
}

func (r *sqliteRepo) InsertUser(ctx context.Context, p InsertUserParams) (models.User, error) {
	u, err := r.q.CreateUser(ctx, gen.CreateUserParams{
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

func (r *sqliteRepo) GetAuthRecordByUsername(ctx context.Context, username string) (AuthRecord, error) {
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
