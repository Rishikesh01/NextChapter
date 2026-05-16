package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/models"
	"github.com/enable-it/nextchapter/backend/internal/users"
)

// AuthService is the surface the HTTP handlers consume for auth-token
// lifecycle (session create/delete on login/logout, API token create/delete
// for the extension) and for credential verification. Resolve / Touch
// are middleware-internal and stay off this interface. Method names
// are conventional infrastructure verbs qualified by the resource noun
// (CreateSession / DeleteSession / CreateAPIToken / DeleteAPIToken) so
// each declaration is self-documenting at the interface, not the call
// site. Auth is an infra service, not a product-domain service, so it
// does not get the product-verb treatment that series / entries get.
type AuthService interface {
	CreateSession(ctx context.Context, userID int64) (models.SessionToken, error)
	DeleteSession(ctx context.Context, rawToken string) error
	CreateAPIToken(ctx context.Context, userID int64, token models.NewToken) (models.APIToken, error)
	DeleteAPIToken(ctx context.Context, userID, tokenID int64) (bool, error)
	Authenticate(ctx context.Context, creds models.Credentials) (models.User, error)
}

// Service owns mint/revoke flows over auth_tokens plus the read-side
// resolve/touch that the middleware in this package depends on. It
// also runs the bcrypt-verify step for [Service.Authenticate]: the
// users repository hands back a [users.AuthRecord] with the stored
// hash and this service does the compare so the password-hash
// boundary lives in one place.
//
// All SQL access goes through [Repository] and [users.Repository];
// this type does not import the sqlc-generated package directly.
type Service struct {
	repo   Repository
	users  users.Repository
	logger *zap.Logger
}

// Compile-time check: the concrete Service satisfies the
// AuthService surface that handlers consume.
var _ AuthService = (*Service)(nil)

// NewService constructs a Service. The users repository is read by
// [Service.Authenticate] to fetch the stored bcrypt hash; if you are
// wiring a test fixture that never calls Authenticate, passing nil is
// fine. Passing a nil logger is also fine; a no-op logger is
// substituted, which is what the integration tests do.
func NewService(repo Repository, userRepo users.Repository, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{repo: repo, users: userRepo, logger: logger}
}

// CreateSession mints a session token, stores its hash, and returns
// the raw token to the caller (who must put it in a Set-Cookie header).
func (s *Service) CreateSession(ctx context.Context, userID int64) (models.SessionToken, error) {
	raw, err := MintToken(constants.TokenKindSession)
	if err != nil {
		s.logger.Error("create session: mint token",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return models.SessionToken{}, err
	}
	now := time.Now().UTC()
	exp := now.Add(SessionDuration)
	row, err := s.repo.CreateToken(ctx, InsertTokenParams{
		UserID:     userID,
		Kind:       constants.TokenKindSession,
		TokenHash:  HashToken(raw),
		CreatedAt:  now,
		LastUsedAt: &now,
		ExpiresAt:  &exp,
	})
	if err != nil {
		s.logger.Error("create session: persist token",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return models.SessionToken{}, fmt.Errorf("auth: create session token: %w", err)
	}
	s.logger.Info("session created",
		zap.Int64("user_id", userID),
		zap.Int64("token_id", row.ID),
		zap.Timep("expires_at", &exp),
	)
	return models.SessionToken{Raw: raw, Token: row}, nil
}

// CreateAPIToken mints a user-labelled bearer token for the extension.
// token.ExpiresAt may be nil (= never expires). The returned
// [models.APIToken] is the wire shape for POST /auth/tokens: the
// stored row fields plus the raw plaintext token, which the server
// surfaces exactly once.
func (s *Service) CreateAPIToken(ctx context.Context, userID int64, token models.NewToken) (models.APIToken, error) {
	raw, err := MintToken(constants.TokenKindAPI)
	if err != nil {
		s.logger.Error("create api token: mint",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return models.APIToken{}, err
	}
	now := time.Now().UTC()
	var exp *time.Time
	if token.ExpiresAt != nil {
		v := token.ExpiresAt.UTC()
		exp = &v
	}
	row, err := s.repo.CreateToken(ctx, InsertTokenParams{
		UserID:     userID,
		Kind:       constants.TokenKindAPI,
		TokenHash:  HashToken(raw),
		Label:      strings.TrimSpace(token.Label),
		LabelValid: true,
		CreatedAt:  now,
		ExpiresAt:  exp,
	})
	if err != nil {
		s.logger.Error("create api token: persist",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return models.APIToken{}, fmt.Errorf("auth: create api token: %w", err)
	}
	s.logger.Info("api token minted",
		zap.Int64("user_id", userID),
		zap.Int64("token_id", row.ID),
		zap.String("label", row.Label),
		zap.Timep("expires_at", row.ExpiresAt),
	)
	return models.APIToken{
		ID:         row.ID,
		Label:      row.Label,
		CreatedAt:  row.CreatedAt,
		LastUsedAt: row.LastUsedAt,
		ExpiresAt:  row.ExpiresAt,
		Raw:        raw,
	}, nil
}

// DeleteAPIToken revokes an API token. Returns false if no row was
// matched (handler turns this into 404).
func (s *Service) DeleteAPIToken(ctx context.Context, userID, tokenID int64) (bool, error) {
	n, err := s.repo.DeleteTokenByID(ctx, userID, tokenID)
	if err != nil {
		s.logger.Error("delete api token",
			zap.Int64("user_id", userID),
			zap.Int64("token_id", tokenID),
			zap.Error(err),
		)
		return false, err
	}
	matched := n > 0
	s.logger.Info("api token revoked",
		zap.Int64("user_id", userID),
		zap.Int64("token_id", tokenID),
		zap.Bool("matched", matched),
	)
	return matched, nil
}

// DeleteSession invalidates a session token by its raw value. Used by
// /auth/logout. Returns nil if the row was already gone.
func (s *Service) DeleteSession(ctx context.Context, rawToken string) error {
	if err := s.repo.DeleteTokenByHash(ctx, HashToken(rawToken)); err != nil {
		s.logger.Error("delete session", zap.Error(err))
		return err
	}
	s.logger.Info("session deleted")
	return nil
}

// Authenticate verifies credentials against the stored bcrypt hash
// and returns the public-facing [models.User] on success. The
// password hash is read from [users.Repository.GetAuthRecordByUsername]
// here and immediately discarded — only this method ever sees it on
// the live path.
//
// [models.ErrInvalidCredentials] is returned on a bcrypt mismatch;
// [models.ErrUserNotFound] when the username does not exist. Handlers
// collapse both into the same 401 envelope so callers cannot
// enumerate accounts.
func (s *Service) Authenticate(ctx context.Context, creds models.Credentials) (models.User, error) {
	s.logger.Debug("authenticate: lookup user",
		zap.String("username", creds.Username),
	)
	rec, err := s.users.GetAuthRecordByUsername(ctx, creds.Username)
	if err != nil {
		// Username miss is benign-but-observable (operator wants the
		// audit trail). Never log the password.
		if errors.Is(err, models.ErrUserNotFound) {
			s.logger.Warn("authentication failed: user not found",
				zap.String("username", creds.Username),
			)
		} else {
			s.logger.Error("authenticate: lookup",
				zap.String("username", creds.Username),
				zap.Error(err),
			)
		}
		return models.User{}, err
	}
	if err := VerifyPassword(rec.PasswordHash, creds.Password); err != nil {
		// Surface the canonical sentinel so handlers can errors.Is
		// without importing this package.
		if errors.Is(err, ErrInvalidCredentials) {
			s.logger.Warn("authentication failed: invalid credentials",
				zap.String("username", creds.Username),
			)
			return models.User{}, models.ErrInvalidCredentials
		}
		s.logger.Error("authenticate: bcrypt compare",
			zap.String("username", creds.Username),
			zap.Error(err),
		)
		return models.User{}, err
	}
	s.logger.Info("authenticated",
		zap.Int64("user_id", rec.ID),
		zap.String("username", rec.Username),
	)
	return models.User{
		ID:        rec.ID,
		Username:  rec.Username,
		CreatedAt: rec.CreatedAt,
	}, nil
}

// Resolve looks up a raw token, returns the owning user and token
// metadata, and rejects expired rows. It does *not* mutate
// last_used_at; the middleware calls [Service.Touch] after a
// successful authorisation. Hashing happens here so callers do not
// need to remember to hash themselves.
func (s *Service) Resolve(ctx context.Context, rawToken string, now time.Time) (Resolved, error) {
	tokHash := HashToken(rawToken)
	row, err := s.repo.GetTokenByHash(ctx, tokHash)
	if err != nil {
		return Resolved{}, err
	}
	if row.Token.ExpiresAt != nil && !row.Token.ExpiresAt.After(now) {
		return Resolved{}, ErrTokenNotFound
	}
	// Defence-in-depth: prefix on the wire must match the stored kind.
	switch row.Token.Kind {
	case constants.TokenKindSession:
		if !strings.HasPrefix(rawToken, constants.TokenPrefixSession) {
			return Resolved{}, ErrTokenNotFound
		}
	case constants.TokenKindAPI:
		if !strings.HasPrefix(rawToken, constants.TokenPrefixAPI) {
			return Resolved{}, ErrTokenNotFound
		}
	default:
		return Resolved{}, fmt.Errorf("auth: unknown stored kind %q", row.Token.Kind)
	}
	return Resolved{
		User: models.User{
			ID:        row.UserID,
			Username:  row.Username,
			CreatedAt: row.UserCreated,
		},
		TokenID:    row.Token.ID,
		Kind:       row.Token.Kind,
		ExpiresAt:  row.Token.ExpiresAt,
		LastUsedAt: row.Token.LastUsedAt,
	}, nil
}

// Touch updates last_used_at and, for session tokens, extends
// expires_at by [SessionDuration] from now. API tokens have their
// last_used_at touched but their expires_at is left alone
// (user-configured).
func (s *Service) Touch(ctx context.Context, r Resolved, now time.Time) error {
	params := TouchParams{ID: r.TokenID, LastUsedAt: &now}
	if r.Kind == constants.TokenKindSession {
		ext := now.Add(SessionDuration)
		params.ExpiresAt = &ext
	}
	return s.repo.TouchToken(ctx, params)
}
