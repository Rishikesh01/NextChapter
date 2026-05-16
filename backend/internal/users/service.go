// Package users owns the user-account domain: creation, lookup, and
// the once-only bootstrap path described in ADR-0006.
package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
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
//
// Method names are conventional user-management verbs (Register /
// CountUsers) rather than product-domain verbs — users is an infra
// service, not a product-domain service. Method-name qualification
// (CountUsers, not bare Count) keeps the interface declaration
// self-documenting in isolation.
type UsersService interface {
	CountUsers(ctx context.Context) (int64, error)
	Register(ctx context.Context, registration models.Registration) (models.User, error)
}

// Service owns the users domain. The password hash is written here on
// [Service.Register] and read from the repository by
// [internal/auth].Service.Authenticate via [Repository.GetAuthRecordByUsername].
// Nothing else in the codebase reads PasswordHash.
//
// We bcrypt-hash here directly (rather than calling [internal/auth].HashPassword)
// so this package does not import internal/auth: the inverse arrow —
// auth depending on users.Repository for credential lookup — is the
// one we want, and a mutual import would not compile.
type Service struct {
	repo   Repository
	logger *zap.Logger
}

// Compile-time check: the concrete Service satisfies the
// UsersService surface that handlers consume.
var _ UsersService = (*Service)(nil)

// NewService constructs a Service. Passing a nil logger is fine for
// tests that don't care about log output; a no-op logger is substituted.
// Real wiring in [internal/server] passes the root zap logger.
func NewService(repo Repository, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{repo: repo, logger: logger}
}

// CountUsers returns the total number of users. Used by /auth/register to
// decide whether the open-registration window is still open.
func (s *Service) CountUsers(ctx context.Context) (int64, error) {
	return s.repo.CountUsers(ctx)
}

// Register hashes the password and inserts a new user. Returns the
// public-facing [models.User] (no hash).
func (s *Service) Register(ctx context.Context, registration models.Registration) (models.User, error) {
	s.logger.Debug("register: hashing password",
		zap.String("username", registration.Username),
	)
	h, err := bcrypt.GenerateFromPassword([]byte(registration.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("register: hash password",
			zap.String("username", registration.Username),
			zap.Error(err),
		)
		return models.User{}, fmt.Errorf("users: hash password: %w", err)
	}
	now := time.Now().UTC()
	u, err := s.repo.InsertUser(ctx, InsertUserParams{
		Username:     registration.Username,
		PasswordHash: string(h),
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		// Username-taken is a normal, caller-driven outcome — Info, not
		// Error. Anything else is a real failure.
		if errors.Is(err, models.ErrUsernameTaken) {
			s.logger.Info("register: username taken",
				zap.String("username", registration.Username),
			)
		} else {
			s.logger.Error("register: insert user",
				zap.String("username", registration.Username),
				zap.Error(err),
			)
		}
		return models.User{}, err
	}
	s.logger.Info("user registered",
		zap.Int64("user_id", u.ID),
		zap.String("username", u.Username),
	)
	return u, nil
}
