package sites

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
)

// insertSiteRuleParams is the input for [Repository.insertSiteRule].
type insertSiteRuleParams struct {
	UserID              int64
	Host                string
	ChapterURLRegex     string
	SlugCaptureGroup    string
	ChapterCaptureGroup string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// updateSiteRuleParams is the input for [Repository.updateSiteRule].
type updateSiteRuleParams struct {
	ID                  int64
	UserID              int64
	Host                string
	ChapterURLRegex     string
	SlugCaptureGroup    string
	ChapterCaptureGroup string
	UpdatedAt           time.Time
}

// Repository is the persistence surface for the sites domain. The
// service in this package depends on this interface; the concrete
// implementations in Repository_sqlite.go and Repository_postgres.go
// are the only things in the package that import sqlc-generated code.
type Repository interface {
	insertSiteRule(ctx context.Context, p insertSiteRuleParams) (models.SiteRule, error)
	getSiteRuleByID(ctx context.Context, userID, ruleID int64) (models.SiteRule, error)
	getSiteRuleByHost(ctx context.Context, userID int64, host string) (models.SiteRule, error)
	listSiteRulesByUser(ctx context.Context, userID int64) ([]models.SiteRule, error)
	updateSiteRule(ctx context.Context, p updateSiteRuleParams) (models.SiteRule, error)
	deleteSiteRule(ctx context.Context, userID, ruleID int64) (int64, error)
}
