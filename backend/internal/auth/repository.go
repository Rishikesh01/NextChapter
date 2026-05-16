package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// NewRepository builds the concrete Repository backed by a *gen.Queries.
func NewRepository(q *gen.Queries) Repository {
	return &repository{q: q}
}

func (r *repository) CreateToken(ctx context.Context, p InsertTokenParams) (models.Token, error) {
	var label sql.NullString
	if p.LabelValid {
		label = sql.NullString{String: p.Label, Valid: true}
	}
	row, err := r.q.CreateAuthToken(ctx, gen.CreateAuthTokenParams{
		UserID:     p.UserID,
		Kind:       p.Kind,
		TokenHash:  p.TokenHash,
		Label:      label,
		CreatedAt:  p.CreatedAt,
		LastUsedAt: timePtrToNullTime(p.LastUsedAt),
		ExpiresAt:  timePtrToNullTime(p.ExpiresAt),
	})
	if err != nil {
		return models.Token{}, fmt.Errorf("auth: create token: %w", err)
	}
	return tokenFromGen(row), nil
}

func (r *repository) GetTokenByHash(ctx context.Context, tokenHash string) (LookupRow, error) {
	row, err := r.q.GetAuthTokenByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LookupRow{}, ErrTokenNotFound
		}
		return LookupRow{}, fmt.Errorf("auth: lookup token: %w", err)
	}
	return LookupRow{
		Token: models.Token{
			ID:         row.ID,
			UserID:     row.UserID,
			Kind:       row.Kind,
			TokenHash:  row.TokenHash,
			Label:      row.Label.String,
			LabelValid: row.Label.Valid,
			CreatedAt:  row.CreatedAt,
			LastUsedAt: nullTimeToPtr(row.LastUsedAt),
			ExpiresAt:  nullTimeToPtr(row.ExpiresAt),
		},
		UserID:       row.UserID,
		Username:     row.UserUsername,
		PasswordHash: row.UserPasswordHash,
		UserCreated:  row.UserCreatedAt,
		UserUpdated:  row.UserUpdatedAt,
	}, nil
}

func (r *repository) TouchToken(ctx context.Context, p TouchParams) error {
	err := r.q.TouchAuthToken(ctx, gen.TouchAuthTokenParams{
		LastUsedAt: timePtrToNullTime(p.LastUsedAt),
		ExpiresAt:  timePtrToNullTime(p.ExpiresAt),
		ID:         p.ID,
	})
	if err != nil {
		return fmt.Errorf("auth: touch token: %w", err)
	}
	return nil
}

func (r *repository) DeleteTokenByID(ctx context.Context, userID, tokenID int64) (int64, error) {
	n, err := r.q.DeleteAuthTokenByID(ctx, gen.DeleteAuthTokenByIDParams{
		ID:     tokenID,
		UserID: userID,
	})
	if err != nil {
		return 0, fmt.Errorf("auth: delete token by id: %w", err)
	}
	return n, nil
}

func (r *repository) DeleteTokenByHash(ctx context.Context, tokenHash string) error {
	if err := r.q.DeleteAuthTokenByHash(ctx, tokenHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("auth: delete token by hash: %w", err)
	}
	return nil
}

func (r *repository) ListAPITokens(ctx context.Context, userID int64) ([]models.Token, error) {
	rows, err := r.q.ListAPITokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("auth: list api tokens: %w", err)
	}
	out := make([]models.Token, 0, len(rows))
	for _, row := range rows {
		out = append(out, tokenFromGen(row))
	}
	return out, nil
}

func (r *repository) ListSessionTokens(ctx context.Context, userID int64) ([]models.Token, error) {
	rows, err := r.q.ListSessionTokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("auth: list session tokens: %w", err)
	}
	out := make([]models.Token, 0, len(rows))
	for _, row := range rows {
		out = append(out, tokenFromGen(row))
	}
	return out, nil
}

// --- conversion helpers --------------------------------------------------

func tokenFromGen(t gen.AuthToken) models.Token {
	return models.Token{
		ID:         t.ID,
		UserID:     t.UserID,
		Kind:       t.Kind,
		TokenHash:  t.TokenHash,
		Label:      t.Label.String,
		LabelValid: t.Label.Valid,
		CreatedAt:  t.CreatedAt,
		LastUsedAt: nullTimeToPtr(t.LastUsedAt),
		ExpiresAt:  nullTimeToPtr(t.ExpiresAt),
	}
}

func nullTimeToPtr(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	v := n.Time
	return &v
}

func timePtrToNullTime(p *time.Time) sql.NullTime {
	if p == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *p, Valid: true}
}
