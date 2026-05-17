package sites

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

func (r *postgresRepo) InsertSiteRule(ctx context.Context, p InsertSiteRuleParams) (models.SiteRule, error) {
	row, err := r.q.InsertSiteRule(ctx, pg.InsertSiteRuleParams{
		UserID:              p.UserID,
		Host:                p.Host,
		ChapterUrlRegex:     p.ChapterURLRegex,
		SlugCaptureGroup:    p.SlugCaptureGroup,
		ChapterCaptureGroup: p.ChapterCaptureGroup,
		CreatedAt:           p.CreatedAt,
		UpdatedAt:           p.UpdatedAt,
	})
	if err != nil {
		return models.SiteRule{}, fmt.Errorf("sites: insert: %w", err)
	}
	return siteRuleFromPostgres(row), nil
}

func (r *postgresRepo) GetSiteRuleByID(ctx context.Context, userID, ruleID int64) (models.SiteRule, error) {
	row, err := r.q.GetSiteRuleByID(ctx, pg.GetSiteRuleByIDParams{ID: ruleID, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.SiteRule{}, models.ErrSiteRuleNotFound
		}
		return models.SiteRule{}, fmt.Errorf("sites: get by id: %w", err)
	}
	return siteRuleFromPostgres(row), nil
}

func (r *postgresRepo) GetSiteRuleByHost(ctx context.Context, userID int64, host string) (models.SiteRule, error) {
	row, err := r.q.GetSiteRuleByHost(ctx, pg.GetSiteRuleByHostParams{UserID: userID, Host: host})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.SiteRule{}, models.ErrSiteRuleNotFound
		}
		return models.SiteRule{}, fmt.Errorf("sites: get by host: %w", err)
	}
	return siteRuleFromPostgres(row), nil
}

func (r *postgresRepo) ListSiteRulesByUser(ctx context.Context, userID int64) ([]models.SiteRule, error) {
	rows, err := r.q.ListSiteRulesByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("sites: list by user: %w", err)
	}
	out := make([]models.SiteRule, 0, len(rows))
	for _, row := range rows {
		out = append(out, siteRuleFromPostgres(row))
	}
	return out, nil
}

func (r *postgresRepo) UpdateSiteRule(ctx context.Context, p UpdateSiteRuleParams) (models.SiteRule, error) {
	row, err := r.q.UpdateSiteRule(ctx, pg.UpdateSiteRuleParams{
		Host:                p.Host,
		ChapterUrlRegex:     p.ChapterURLRegex,
		SlugCaptureGroup:    p.SlugCaptureGroup,
		ChapterCaptureGroup: p.ChapterCaptureGroup,
		UpdatedAt:           p.UpdatedAt,
		ID:                  p.ID,
		UserID:              p.UserID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.SiteRule{}, models.ErrSiteRuleNotFound
		}
		return models.SiteRule{}, fmt.Errorf("sites: update: %w", err)
	}
	return siteRuleFromPostgres(row), nil
}

func (r *postgresRepo) DeleteSiteRule(ctx context.Context, userID, ruleID int64) (int64, error) {
	n, err := r.q.DeleteSiteRule(ctx, pg.DeleteSiteRuleParams{ID: ruleID, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("sites: delete: %w", err)
	}
	return n, nil
}

func siteRuleFromPostgres(r pg.SiteRule) models.SiteRule {
	return models.SiteRule{
		ID:                  r.ID,
		UserID:              r.UserID,
		Host:                r.Host,
		ChapterURLRegex:     r.ChapterUrlRegex,
		SlugCaptureGroup:    r.SlugCaptureGroup,
		ChapterCaptureGroup: r.ChapterCaptureGroup,
		CreatedAt:           r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
	}
}
