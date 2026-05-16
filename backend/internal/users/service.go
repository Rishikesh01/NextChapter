// Package users owns the user-account domain: creation, lookup, and
// the once-only bootstrap path described in ADR-0006.
package users

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

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

// UsersService is the surface the HTTP handlers consume for user
// account lifecycle. Credential verification lives on
// [internal/auth].Service.Authenticate, not here: the users service
// keeps account-lifecycle methods only. Keeping Authenticate off this
// interface lets [internal/auth] own the password-hash boundary
// (matching where token mint/verify also live).
type UsersService interface {
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, p models.Registration) (models.User, error)
}

// Service owns the users domain. The password hash is written here on
// [Service.Create] and read from the repository by
// [internal/auth].Service.Authenticate via [Repository.GetAuthRecordByUsername].
// Nothing else in the codebase reads PasswordHash.
//
// We bcrypt-hash here directly (rather than calling [internal/auth].HashPassword)
// so this package does not import internal/auth: the inverse arrow —
// auth depending on users.Repository for credential lookup — is the
// one we want, and a mutual import would not compile.
type Service struct {
	repo Repository
}

// Compile-time check: the concrete Service satisfies the
// UsersService surface that handlers consume.
var _ UsersService = (*Service)(nil)

// NewService constructs a Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Count returns the total number of users. Used by /auth/register to
// decide whether the open-registration window is still open.
func (s *Service) Count(ctx context.Context) (int64, error) {
	return s.repo.CountUsers(ctx)
}

// Create hashes the password and inserts a new user. Returns the
// public-facing [models.User] (no hash).
func (s *Service) Create(ctx context.Context, p models.Registration) (models.User, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, fmt.Errorf("users: hash password: %w", err)
	}
	now := time.Now().UTC()
	return s.repo.InsertUser(ctx, InsertUserParams{
		Username:     p.Username,
		PasswordHash: string(h),
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}
