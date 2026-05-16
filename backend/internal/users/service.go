// Package users owns the user-account domain: creation, lookup, and the
// once-only bootstrap path described in ADR-0006.
package users

import (
	"context"
	"errors"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/auth"
)

// ErrUsernameTaken is returned when CreateUser collides on the username
// UNIQUE constraint. Handlers turn this into a 422.
var ErrUsernameTaken = errors.New("users: username already taken")

// ErrNotFound is returned when a lookup misses.
var ErrNotFound = errors.New("users: not found")

// NewService constructs a Service. If now is nil, time.Now is used.
func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, now: now}
}

// Count returns the total number of users. Used by /auth/register to
// decide whether the open-registration window is still open.
func (s *Service) Count(ctx context.Context) (int64, error) {
	return s.repo.CountUsers(ctx)
}

// Create hashes the password and inserts a new user.
func (s *Service) Create(ctx context.Context, p RegisterParams) (User, error) {
	hash, err := auth.HashPassword(p.Password)
	if err != nil {
		return User{}, err
	}
	now := s.now().UTC()
	return s.repo.InsertUser(ctx, InsertUserParams{
		Username:     p.Username,
		PasswordHash: hash,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}

// GetByUsername looks up a user by their unique username.
func (s *Service) GetByUsername(ctx context.Context, username string) (User, error) {
	return s.repo.GetUserByUsername(ctx, username)
}
