package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/enable-it/nextchapter/backend/internal/models"
	pg "github.com/enable-it/nextchapter/backend/internal/store/generated/pg"
)

type postgresRepo struct {
	q *pg.Queries
}

func newPostgresRepository(db *sql.DB) *postgresRepo {
	return &postgresRepo{q: pg.New(db)}
}

func (r *postgresRepo) CreateToken(ctx context.Context, p InsertTokenParams) (models.Token, error) {
	var label sql.NullString
	if p.LabelValid {
		label = sql.NullString{String: p.Label, Valid: true}
	}
	row, err := r.q.CreateAuthToken(ctx, pg.CreateAuthTokenParams{
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
	return tokenFromPostgres(row), nil
}

func (r *postgresRepo) GetTokenByHash(ctx context.Context, tokenHash string) (LookupRow, error) {
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
		UserID:      row.UserID,
		Username:    row.UserUsername,
		UserCreated: row.UserCreatedAt,
	}, nil
}

func (r *postgresRepo) DeleteTokenByID(ctx context.Context, userID, tokenID int64) (int64, error) {
	n, err := r.q.DeleteAuthTokenByID(ctx, pg.DeleteAuthTokenByIDParams{
		ID:     tokenID,
		UserID: userID,
	})
	if err != nil {
		return 0, fmt.Errorf("auth: delete token by id: %w", err)
	}
	return n, nil
}

func (r *postgresRepo) DeleteTokenByHash(ctx context.Context, tokenHash string) error {
	if err := r.q.DeleteAuthTokenByHash(ctx, tokenHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("auth: delete token by hash: %w", err)
	}
	return nil
}

func tokenFromPostgres(t pg.AuthToken) models.Token {
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
