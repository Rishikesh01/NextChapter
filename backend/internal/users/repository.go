package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// NewRepository builds the concrete Repository backed by a *gen.Queries.
func NewRepository(q *gen.Queries) Repository {
	return &repository{q: q}
}

func (r *repository) InsertUser(ctx context.Context, p InsertUserParams) (models.User, error) {
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

func (r *repository) GetUserByID(ctx context.Context, id int64) (models.User, error) {
	u, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, models.ErrUserNotFound
		}
		return models.User{}, fmt.Errorf("users: get by id: %w", err)
	}
	return models.User{
		ID:        u.ID,
		Username:  u.Username,
		CreatedAt: u.CreatedAt,
	}, nil
}

func (r *repository) GetAuthRecordByUsername(ctx context.Context, username string) (AuthRecord, error) {
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

func (r *repository) CountUsers(ctx context.Context) (int64, error) {
	n, err := r.q.CountUsers(ctx)
	if err != nil {
		return 0, fmt.Errorf("users: count: %w", err)
	}
	return n, nil
}

// isUniqueViolation detects SQLite "UNIQUE constraint failed" errors.
// We deliberately match on the error string because modernc.org/sqlite
// surfaces an internal error type we'd rather not depend on.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "UNIQUE constraint failed") || strings.Contains(s, "constraint failed: UNIQUE")
}
