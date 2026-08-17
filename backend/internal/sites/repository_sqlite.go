package sites

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

type sqliteRepo struct {
	q *gen.Queries
}

func NewSQLiteRepository(db *sql.DB) *sqliteRepo {
	return &sqliteRepo{q: gen.New(db)}
}

func (r *sqliteRepo) insertSiteRule(ctx context.Context, p insertSiteRuleParams) (models.SiteRule, error) {
	row, err := r.q.InsertSiteRule(ctx, gen.InsertSiteRuleParams{
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
	return siteRuleFromSQLite(row), nil
}

func (r *sqliteRepo) getSiteRuleByID(ctx context.Context, userID, ruleID int64) (models.SiteRule, error) {
	row, err := r.q.GetSiteRuleByID(ctx, gen.GetSiteRuleByIDParams{ID: ruleID, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.SiteRule{}, models.ErrSiteRuleNotFound
		}
		return models.SiteRule{}, fmt.Errorf("sites: get by id: %w", err)
	}
	return siteRuleFromSQLite(row), nil
}

func (r *sqliteRepo) getSiteRuleByHost(ctx context.Context, userID int64, host string) (models.SiteRule, error) {
	row, err := r.q.GetSiteRuleByHost(ctx, gen.GetSiteRuleByHostParams{UserID: userID, Host: host})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.SiteRule{}, models.ErrSiteRuleNotFound
		}
		return models.SiteRule{}, fmt.Errorf("sites: get by host: %w", err)
	}
	return siteRuleFromSQLite(row), nil
}

func (r *sqliteRepo) listSiteRulesByUser(ctx context.Context, userID int64) ([]models.SiteRule, error) {
	rows, err := r.q.ListSiteRulesByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("sites: list by user: %w", err)
	}
	out := make([]models.SiteRule, 0, len(rows))
	for _, row := range rows {
		out = append(out, siteRuleFromSQLite(row))
	}
	return out, nil
}

func (r *sqliteRepo) updateSiteRule(ctx context.Context, p updateSiteRuleParams) (models.SiteRule, error) {
	row, err := r.q.UpdateSiteRule(ctx, gen.UpdateSiteRuleParams{
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
	return siteRuleFromSQLite(row), nil
}

func (r *sqliteRepo) deleteSiteRule(ctx context.Context, userID, ruleID int64) (int64, error) {
	n, err := r.q.DeleteSiteRule(ctx, gen.DeleteSiteRuleParams{ID: ruleID, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("sites: delete: %w", err)
	}
	return n, nil
}

func siteRuleFromSQLite(r gen.SiteRule) models.SiteRule {
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
