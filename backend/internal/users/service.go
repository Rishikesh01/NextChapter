// Package users owns the user-account domain: creation, lookup, and
// the once-only bootstrap path described in ADR-0006.
package users

import (
	"context"
	"errors"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/auth"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// Re-exported sentinels so callers inside this package (and the
// integration tests, historically) can use the short name. The
// canonical values live in [models] so handlers can errors.Is without
// importing this package.
var (
	ErrUsernameTaken = models.ErrUsernameTaken
	ErrNotFound      = models.ErrUserNotFound
)

// Service owns the users domain. It is the only thing in the
// codebase that touches PasswordHash on the live path: handlers
// receive [models.User] (no hash) back from [Service.Authenticate]
// and never see authRecord.
type Service struct {
	repo Repository
	now  func() time.Time
}

// Compile-time check: the concrete Service satisfies the
// models.UsersService surface that handlers consume.
var _ models.UsersService = (*Service)(nil)

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

// Create hashes the password and inserts a new user. Returns the
// public-facing [models.User] (no hash).
func (s *Service) Create(ctx context.Context, p models.Registration) (models.User, error) {
	hash, err := auth.HashPassword(p.Password)
	if err != nil {
		return models.User{}, err
	}
	now := s.now().UTC()
	return s.repo.InsertUser(ctx, InsertUserParams{
		Username:     p.Username,
		PasswordHash: hash,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}

// Authenticate verifies credentials against the stored bcrypt hash
// and returns the public-facing [models.User] on success.
// [models.ErrInvalidCredentials] is returned on a bcrypt mismatch;
// [models.ErrUserNotFound] when the username does not exist. Handlers
// collapse both into the same 401 envelope so callers cannot
// enumerate accounts.
func (s *Service) Authenticate(ctx context.Context, p models.Credentials) (models.User, error) {
	rec, err := s.repo.GetAuthRecordByUsername(ctx, p.Username)
	if err != nil {
		return models.User{}, err
	}
	if err := auth.VerifyPassword(rec.PasswordHash, p.Password); err != nil {
		// The auth package returns its own sentinel; surface the
		// canonical one so handlers can errors.Is without an auth
		// import.
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return models.User{}, models.ErrInvalidCredentials
		}
		return models.User{}, err
	}
	return models.User{
		ID:        rec.ID,
		Username:  rec.Username,
		CreatedAt: rec.CreatedAt,
	}, nil
}
