package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// Service owns mint/revoke flows over auth_tokens plus the read-side
// resolve/touch that the middleware in this package depends on. All
// SQL access goes through [Repository]; this type does not import the
// sqlc-generated package directly.
type Service struct {
	repo Repository
	now  func() time.Time
}

// Compile-time check: the concrete Service satisfies the
// models.AuthService surface that handlers consume.
var _ models.AuthService = (*Service)(nil)

// NewService constructs a Service. If now is nil, time.Now is used.
func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, now: now}
}

// CreateSession mints a session token, stores its hash, and returns
// the raw token to the caller (who must put it in a Set-Cookie header).
func (s *Service) CreateSession(ctx context.Context, userID int64) (models.SessionToken, error) {
	raw, err := MintToken(constants.TokenKindSession)
	if err != nil {
		return models.SessionToken{}, err
	}
	now := s.now().UTC()
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
		return models.SessionToken{}, fmt.Errorf("auth: create session token: %w", err)
	}
	return models.SessionToken{Raw: raw, Token: row}, nil
}

// CreateAPI mints a user-labelled bearer token for the extension.
// p.ExpiresAt may be nil (= never expires).
func (s *Service) CreateAPI(ctx context.Context, userID int64, p models.NewToken) (models.APIToken, error) {
	raw, err := MintToken(constants.TokenKindAPI)
	if err != nil {
		return models.APIToken{}, err
	}
	now := s.now().UTC()
	var exp *time.Time
	if p.ExpiresAt != nil {
		v := p.ExpiresAt.UTC()
		exp = &v
	}
	row, err := s.repo.CreateToken(ctx, InsertTokenParams{
		UserID:     userID,
		Kind:       constants.TokenKindAPI,
		TokenHash:  HashToken(raw),
		Label:      strings.TrimSpace(p.Label),
		LabelValid: true,
		CreatedAt:  now,
		ExpiresAt:  exp,
	})
	if err != nil {
		return models.APIToken{}, fmt.Errorf("auth: create api token: %w", err)
	}
	return models.APIToken{Raw: raw, Token: row}, nil
}

// DeleteAPI revokes an API token. Returns false if no row was matched
// (handler turns this into 404).
func (s *Service) DeleteAPI(ctx context.Context, userID, tokenID int64) (bool, error) {
	n, err := s.repo.DeleteTokenByID(ctx, userID, tokenID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// DeleteSession invalidates a session token by its raw value. Used by
// /auth/logout. Returns nil if the row was already gone.
func (s *Service) DeleteSession(ctx context.Context, rawToken string) error {
	return s.repo.DeleteTokenByHash(ctx, HashToken(rawToken))
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
	p := TouchParams{ID: r.TokenID, LastUsedAt: &now}
	if r.Kind == constants.TokenKindSession {
		ext := now.Add(SessionDuration)
		p.ExpiresAt = &ext
	}
	return s.repo.TouchToken(ctx, p)
}
