package sites

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
)

// InsertSiteRuleParams is the input for [Repository.InsertSiteRule].
type InsertSiteRuleParams struct {
	UserID              int64
	Host                string
	ChapterURLRegex     string
	SlugCaptureGroup    string
	ChapterCaptureGroup string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// UpdateSiteRuleParams is the input for [Repository.UpdateSiteRule].
type UpdateSiteRuleParams struct {
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
// implementations in repository_sqlite.go and repository_postgres.go
// are the only things in the package that import sqlc-generated code.
type Repository interface {
	InsertSiteRule(ctx context.Context, p InsertSiteRuleParams) (models.SiteRule, error)
	GetSiteRuleByID(ctx context.Context, userID, ruleID int64) (models.SiteRule, error)
	GetSiteRuleByHost(ctx context.Context, userID int64, host string) (models.SiteRule, error)
	ListSiteRulesByUser(ctx context.Context, userID int64) ([]models.SiteRule, error)
	UpdateSiteRule(ctx context.Context, p UpdateSiteRuleParams) (models.SiteRule, error)
	DeleteSiteRule(ctx context.Context, userID, ruleID int64) (int64, error)
}
