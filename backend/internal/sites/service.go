// Package sites owns the per-user site-rule CRUD surface plus the
// compiled-in seed list ([Defaults]) that is copied into the
// site_rule table when a new user registers. The browser extension
// uses these rules client-side to detect which URLs are chapter
// pages and to extract the series slug + chapter number from the
// path — the server does NOT gate captures on whether a rule exists.
package sites

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/internal/models"
)

// SitesService is the surface the HTTP handlers consume for the
// site-rule CRUD endpoints. Method names are domain verbs qualified
// by the resource noun (AddSiteRule / ListSiteRules / EditSiteRule /
// RemoveSiteRule / SeedSiteRulesForUser) so each declaration is
// self-documenting at the interface, not the call site.
//
// ListSiteRulesAndTrackedHosts is intentionally NOT on this surface:
// the "tracked_hosts" half of GET /sites lives in the entries
// service, and the handler combines the two reads. Keeping sites
// independent of entries avoids the cross-service dep.
type SitesService interface {
	ListSiteRules(ctx context.Context, userID int64) ([]models.SiteRule, error)
	AddSiteRule(ctx context.Context, userID int64, draft models.SiteRuleNew) (models.SiteRule, error)
	EditSiteRule(ctx context.Context, userID, ruleID int64, patch models.SiteRulePatch) (models.SiteRule, error)
	RemoveSiteRule(ctx context.Context, userID, ruleID int64) error
	SeedSiteRulesForUser(ctx context.Context, userID int64) error
}

// MissingCaptureGroupError is the error type returned by
// AddSiteRule / EditSiteRule when one of the configured
// capture-group names is not present in the compiled regex. It
// satisfies errors.Is against [models.ErrSiteRuleMissingCaptureGroup]
// and exposes the JSON field name (slug_capture_group or
// chapter_capture_group) that the handler should attach the
// field-level error to.
type MissingCaptureGroupError struct {
	// JSONField is the wire-level field name of the failed config
	// option: "slug_capture_group" or "chapter_capture_group".
	JSONField string
	// GroupName is the actual capture-group name that was missing
	// from the compiled regex; surfaced in the error string for
	// debugging.
	GroupName string
}

func (e *MissingCaptureGroupError) Error() string {
	return fmt.Sprintf("sites: capture group %q is not present in chapter_url_regex", e.GroupName)
}

func (e *MissingCaptureGroupError) Is(target error) bool {
	return target == models.ErrSiteRuleMissingCaptureGroup
}

// Service exposes the sites domain to handlers.
type Service struct {
	repo   repository
	logger *zap.Logger
}

// Compile-time check: the concrete Service satisfies the
// SitesService surface that handlers consume.
var _ SitesService = (*Service)(nil)

// NewService builds a Service.
func NewService(repo repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// ListSiteRules returns the caller's site-rule rows in host order.
// The slice is always non-nil so the GET /sites wire shape carries
// `rules: []` rather than `rules: null` for a user with no rules.
func (s *Service) ListSiteRules(ctx context.Context, userID int64) ([]models.SiteRule, error) {
	rows, err := s.repo.listSiteRulesByUser(ctx, userID)
	if err != nil {
		s.logger.Error("list site rules",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	if rows == nil {
		rows = []models.SiteRule{}
	}
	return rows, nil
}

// AddSiteRule validates that ChapterURLRegex compiles and that both
// capture-group names appear in the pattern, then inserts a new
// site_rule row. Returns [models.ErrSiteRuleHostTaken] if a rule
// already exists for the (user, host) pair.
func (s *Service) AddSiteRule(ctx context.Context, userID int64, draft models.SiteRuleNew) (models.SiteRule, error) {
	if err := verifyRegexAndCaptures(
		draft.ChapterURLRegex,
		draft.SlugCaptureGroup,
		draft.ChapterCaptureGroup,
	); err != nil {
		s.logger.Info("add site rule rejected: regex/captures",
			zap.Int64("user_id", userID),
			zap.String("host", draft.Host),
			zap.Error(err),
		)
		return models.SiteRule{}, err
	}
	// Pre-check the (user, host) uniqueness rather than rely on the
	// sqlite unique-violation error string: a typed sentinel return
	// is cleaner and the extra round-trip is cheap.
	_, err := s.repo.getSiteRuleByHost(ctx, userID, draft.Host)
	switch {
	case err == nil:
		s.logger.Info("add site rule rejected: host taken",
			zap.Int64("user_id", userID),
			zap.String("host", draft.Host),
		)
		return models.SiteRule{}, models.ErrSiteRuleHostTaken
	case errors.Is(err, models.ErrSiteRuleNotFound):
		// expected — fall through and insert.
	default:
		s.logger.Error("add site rule: host lookup",
			zap.Int64("user_id", userID),
			zap.String("host", draft.Host),
			zap.Error(err),
		)
		return models.SiteRule{}, err
	}
	now := time.Now().UTC()
	row, err := s.repo.insertSiteRule(ctx, insertSiteRuleParams{
		UserID:              userID,
		Host:                draft.Host,
		ChapterURLRegex:     draft.ChapterURLRegex,
		SlugCaptureGroup:    draft.SlugCaptureGroup,
		ChapterCaptureGroup: draft.ChapterCaptureGroup,
		CreatedAt:           now,
		UpdatedAt:           now,
	})
	if err != nil {
		s.logger.Error("add site rule: insert",
			zap.Int64("user_id", userID),
			zap.String("host", draft.Host),
			zap.Error(err),
		)
		return models.SiteRule{}, err
	}
	s.logger.Info("site rule added",
		zap.Int64("user_id", userID),
		zap.Int64("rule_id", row.ID),
		zap.String("host", row.Host),
	)
	return row, nil
}

// EditSiteRule applies a partial patch to a site rule. nil pointer
// fields are left untouched; any non-nil pointer replaces the column
// value. The post-patch regex + capture-group invariant is re-checked
// before persistence so a partial edit can't leave the row in a
// state that fails on read.
func (s *Service) EditSiteRule(ctx context.Context, userID, ruleID int64, patch models.SiteRulePatch) (models.SiteRule, error) {
	current, err := s.repo.getSiteRuleByID(ctx, userID, ruleID)
	if err != nil {
		return models.SiteRule{}, err
	}
	host := current.Host
	if patch.Host != nil {
		host = *patch.Host
	}
	regex := current.ChapterURLRegex
	if patch.ChapterURLRegex != nil {
		regex = *patch.ChapterURLRegex
	}
	slugGroup := current.SlugCaptureGroup
	if patch.SlugCaptureGroup != nil {
		slugGroup = *patch.SlugCaptureGroup
	}
	chapterGroup := current.ChapterCaptureGroup
	if patch.ChapterCaptureGroup != nil {
		chapterGroup = *patch.ChapterCaptureGroup
	}
	if err := verifyRegexAndCaptures(regex, slugGroup, chapterGroup); err != nil {
		s.logger.Info("edit site rule rejected: regex/captures",
			zap.Int64("user_id", userID),
			zap.Int64("rule_id", ruleID),
			zap.Error(err),
		)
		return models.SiteRule{}, err
	}
	// If host is being changed, ensure the new host isn't already
	// taken by a different rule under this user.
	if host != current.Host {
		other, err := s.repo.getSiteRuleByHost(ctx, userID, host)
		switch {
		case err == nil && other.ID != ruleID:
			s.logger.Info("edit site rule rejected: host taken",
				zap.Int64("user_id", userID),
				zap.Int64("rule_id", ruleID),
				zap.String("host", host),
			)
			return models.SiteRule{}, models.ErrSiteRuleHostTaken
		case err != nil && !errors.Is(err, models.ErrSiteRuleNotFound):
			s.logger.Error("edit site rule: host lookup",
				zap.Int64("user_id", userID),
				zap.Int64("rule_id", ruleID),
				zap.String("host", host),
				zap.Error(err),
			)
			return models.SiteRule{}, err
		}
	}
	now := time.Now().UTC()
	row, err := s.repo.updateSiteRule(ctx, updateSiteRuleParams{
		ID:                  ruleID,
		UserID:              userID,
		Host:                host,
		ChapterURLRegex:     regex,
		SlugCaptureGroup:    slugGroup,
		ChapterCaptureGroup: chapterGroup,
		UpdatedAt:           now,
	})
	if err != nil {
		s.logger.Error("edit site rule: update",
			zap.Int64("user_id", userID),
			zap.Int64("rule_id", ruleID),
			zap.Error(err),
		)
		return models.SiteRule{}, err
	}
	s.logger.Info("site rule edited",
		zap.Int64("user_id", userID),
		zap.Int64("rule_id", ruleID),
		zap.String("host", row.Host),
	)
	return row, nil
}

// RemoveSiteRule deletes a site rule. Returns
// [models.ErrSiteRuleNotFound] if no row matched.
func (s *Service) RemoveSiteRule(ctx context.Context, userID, ruleID int64) error {
	n, err := s.repo.deleteSiteRule(ctx, userID, ruleID)
	if err != nil {
		s.logger.Error("remove site rule: delete",
			zap.Int64("user_id", userID),
			zap.Int64("rule_id", ruleID),
			zap.Error(err),
		)
		return err
	}
	if n == 0 {
		s.logger.Info("remove site rule rejected: not found",
			zap.Int64("user_id", userID),
			zap.Int64("rule_id", ruleID),
		)
		return models.ErrSiteRuleNotFound
	}
	s.logger.Info("site rule removed",
		zap.Int64("user_id", userID),
		zap.Int64("rule_id", ruleID),
	)
	return nil
}

// SeedSiteRulesForUser copies [Defaults] into the site_rule table
// for the supplied user. Called from the /auth/register handler and
// from the env-var bootstrap path after the user account is created.
//
// If a rule for one of the default hosts already exists (e.g. the
// caller is re-running an idempotent bootstrap after a partial
// failure) the duplicate is skipped silently — seeding is best
// effort and the existing row is treated as the source of truth.
//
// Returns the first non-skippable error encountered. The bootstrap
// and register paths Warn-log on failure but don't fail the
// surrounding flow: the account is usable without seeded rules.
func (s *Service) SeedSiteRulesForUser(ctx context.Context, userID int64) error {
	if len(Defaults) == 0 {
		return nil
	}
	now := time.Now().UTC()
	seeded := 0
	for _, d := range Defaults {
		// Skip if already present — bootstrap retries shouldn't 422
		// on duplicate-host.
		if _, err := s.repo.getSiteRuleByHost(ctx, userID, d.Host); err == nil {
			continue
		} else if !errors.Is(err, models.ErrSiteRuleNotFound) {
			s.logger.Error("seed: host lookup",
				zap.Int64("user_id", userID),
				zap.String("host", d.Host),
				zap.Error(err),
			)
			return err
		}
		if _, err := s.repo.insertSiteRule(ctx, insertSiteRuleParams{
			UserID:              userID,
			Host:                d.Host,
			ChapterURLRegex:     d.ChapterURLRegex,
			SlugCaptureGroup:    d.SlugCaptureGroup,
			ChapterCaptureGroup: d.ChapterCaptureGroup,
			CreatedAt:           now,
			UpdatedAt:           now,
		}); err != nil {
			s.logger.Error("seed: insert",
				zap.Int64("user_id", userID),
				zap.String("host", d.Host),
				zap.Error(err),
			)
			return err
		}
		seeded++
	}
	s.logger.Info("seeded site rules for user",
		zap.Int64("user_id", userID),
		zap.Int("seeded", seeded),
		zap.Int("total_defaults", len(Defaults)),
	)
	return nil
}

// verifyRegexAndCaptures is the content-aware validation that lives
// in the service layer (where it can be tested without spinning up a
// gin engine) and is invoked from both AddSiteRule and EditSiteRule.
//
// Returns [models.ErrSiteRuleInvalidRegex] when the pattern doesn't
// compile, or a *MissingCaptureGroupError (which errors.Is matches
// [models.ErrSiteRuleMissingCaptureGroup]) when one of the named
// group references isn't actually present in the compiled regex.
// The handler uses errors.As to recover the JSON-field name for the
// 422 field map.
func verifyRegexAndCaptures(pattern, slugGroup, chapterGroup string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return models.ErrSiteRuleInvalidRegex
	}
	names := re.SubexpNames()
	if !containsName(names, slugGroup) {
		return &MissingCaptureGroupError{JSONField: "slug_capture_group", GroupName: slugGroup}
	}
	if !containsName(names, chapterGroup) {
		return &MissingCaptureGroupError{JSONField: "chapter_capture_group", GroupName: chapterGroup}
	}
	return nil
}

func containsName(names []string, target string) bool {
	for _, n := range names {
		if n == target {
			return true
		}
	}
	return false
}
