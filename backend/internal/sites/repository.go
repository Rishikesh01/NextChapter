package sites

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// NewRepository builds the concrete Repository backed by a *gen.Queries.
func NewRepository(q *gen.Queries) Repository {
	return &repository{q: q}
}

func (r *repository) InsertSiteRule(ctx context.Context, p InsertSiteRuleParams) (models.SiteRule, error) {
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
	return siteRuleFromGen(row), nil
}

func (r *repository) GetSiteRuleByID(ctx context.Context, userID, ruleID int64) (models.SiteRule, error) {
	row, err := r.q.GetSiteRuleByID(ctx, gen.GetSiteRuleByIDParams{ID: ruleID, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.SiteRule{}, models.ErrSiteRuleNotFound
		}
		return models.SiteRule{}, fmt.Errorf("sites: get by id: %w", err)
	}
	return siteRuleFromGen(row), nil
}

func (r *repository) GetSiteRuleByHost(ctx context.Context, userID int64, host string) (models.SiteRule, error) {
	row, err := r.q.GetSiteRuleByHost(ctx, gen.GetSiteRuleByHostParams{UserID: userID, Host: host})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.SiteRule{}, models.ErrSiteRuleNotFound
		}
		return models.SiteRule{}, fmt.Errorf("sites: get by host: %w", err)
	}
	return siteRuleFromGen(row), nil
}

func (r *repository) ListSiteRulesByUser(ctx context.Context, userID int64) ([]models.SiteRule, error) {
	rows, err := r.q.ListSiteRulesByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("sites: list by user: %w", err)
	}
	out := make([]models.SiteRule, 0, len(rows))
	for _, row := range rows {
		out = append(out, siteRuleFromGen(row))
	}
	return out, nil
}

func (r *repository) UpdateSiteRule(ctx context.Context, p UpdateSiteRuleParams) (models.SiteRule, error) {
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
	return siteRuleFromGen(row), nil
}

func (r *repository) DeleteSiteRule(ctx context.Context, userID, ruleID int64) (int64, error) {
	n, err := r.q.DeleteSiteRule(ctx, gen.DeleteSiteRuleParams{ID: ruleID, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("sites: delete: %w", err)
	}
	return n, nil
}

func siteRuleFromGen(r gen.SiteRule) models.SiteRule {
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
