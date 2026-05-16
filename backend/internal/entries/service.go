// Package entries owns per-site reading-position rows. Each entry is
// keyed by (user, site_host, series_slug) and is monotonic on
// last_chapter: clicks below the existing value are silently ignored
// (the response is the unchanged row).
package entries

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/internal/models"
)

// Service code does not validate, default, or clamp inputs — that
// belongs to the binding layer (see [models.EntryFilter] for the
// pagination tags). Filter.Limit / Filter.Offset arrive
// already-bounded; this service trusts them and passes them through.

// EntriesService is the surface the HTTP handlers consume for the
// entries endpoints. CaptureChapter returns (entry, created, err) —
// the bool distinguishes the 201 vs 200 paths on the wire. Method
// names are domain verbs qualified by the resource noun
// (CaptureChapter / ListReadingPositions / AdjustReadingPosition /
// ForgetReadingPosition) so each declaration is self-documenting at
// the interface, not the call site.
type EntriesService interface {
	CaptureChapter(ctx context.Context, userID int64, capture models.EntryCapture, sc models.SeriesCreator) (models.Entry, bool, error)
	ListReadingPositions(ctx context.Context, userID int64, filter models.EntryFilter) (models.EntryList, error)
	AdjustReadingPosition(ctx context.Context, userID, entryID int64, patch models.EntryPatch) (models.Entry, error)
	ForgetReadingPosition(ctx context.Context, userID, entryID int64) error
}

// Service exposes the entries domain to handlers.
type Service struct {
	repo   Repository
	logger *zap.Logger
}

// Compile-time check: the concrete Service satisfies the
// EntriesService surface that handlers consume.
var _ EntriesService = (*Service)(nil)

// NewService builds a Service. Passing a nil logger is fine for tests;
// a no-op logger is substituted. The integration tests do exactly that.
func NewService(repo Repository, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{repo: repo, logger: logger}
}

// CaptureChapter implements POST /entries/capture per the openapi spec:
//   - If a row exists for (user, host, slug): advance last_chapter
//     (monotonic — never rewinds) and update last_url / site_title /
//     last_captured_at. 200 OK, idempotent on equal chapter.
//   - Else: create a new row attached to either p.SeriesID (must exist
//     and belong to the user) or a fresh series titled
//     *p.NewSeriesTitle. 201 Created.
//
// The bool return is "was-created": true => 201, false => 200.
func (s *Service) CaptureChapter(ctx context.Context, userID int64, capture models.EntryCapture, sc models.SeriesCreator) (models.Entry, bool, error) {
	s.logger.Debug("capture: lookup existing",
		zap.Int64("user_id", userID),
		zap.String("site_host", capture.SiteHost),
		zap.String("series_slug", capture.SeriesSlug),
	)
	res, err := s.capture(ctx, userID, capture, sc)
	if err != nil {
		// Validation-shaped errors are handler-mapped to 4xx; log them
		// at Info so an operator can correlate a 422 to a service
		// decision without it polluting the error stream.
		switch {
		case errors.Is(err, ErrSeriesRequired), errors.Is(err, ErrSeriesNotFound):
			s.logger.Info("capture rejected",
				zap.Int64("user_id", userID),
				zap.String("site_host", capture.SiteHost),
				zap.String("series_slug", capture.SeriesSlug),
				zap.Error(err),
			)
		default:
			s.logger.Error("capture failed",
				zap.Int64("user_id", userID),
				zap.String("site_host", capture.SiteHost),
				zap.String("series_slug", capture.SeriesSlug),
				zap.Error(err),
			)
		}
		return models.Entry{}, false, err
	}
	s.logger.Info("chapter captured",
		zap.Int64("user_id", userID),
		zap.Int64("series_id", res.Entry.SeriesID),
		zap.Int64("entry_id", res.Entry.ID),
		zap.Bool("created", res.Created),
		zap.Float64("last_chapter", res.Entry.LastChapter),
	)
	return res.Entry, res.Created, nil
}

// capture is the unwrapped core; it returns the package-internal
// captureResult and is kept separate from [Service.CaptureChapter] for
// readability. The public surface is the (entry, created, err) shape
// on the interface.
func (s *Service) capture(ctx context.Context, userID int64, capture models.EntryCapture, sc models.SeriesCreator) (captureResult, error) {
	// capture.Chapter is *float64 because the wire treats 0 as a valid
	// chapter (binding:"required" rejects the missing case). Once
	// we're past ShouldBindJSON it's safe to deref.
	chapter := *capture.Chapter
	existing, err := s.repo.GetEntryByKey(ctx, GetEntryByKeyParams{
		UserID:     userID,
		SiteHost:   capture.SiteHost,
		SeriesSlug: capture.SeriesSlug,
	})
	if err == nil {
		// Advance path. Monotonic on last_chapter.
		now := time.Now().UTC()
		newChapter := existing.LastChapter
		newURL := existing.LastURL
		newTitle := existing.SiteTitle
		if chapter >= existing.LastChapter {
			newChapter = chapter
			newURL = capture.URL
			newTitle = capture.SiteTitle
		}
		// Even a no-op click bumps last_captured_at, per "where was
		// I" product framing: the user clicked again so they're
		// reading it again right now.
		row, err := s.repo.AdvanceEntry(ctx, AdvanceEntryParams{
			ID:             existing.ID,
			UserID:         userID,
			LastChapter:    newChapter,
			LastURL:        newURL,
			SiteTitle:      newTitle,
			LastCapturedAt: now,
			UpdatedAt:      now,
		})
		if err != nil {
			return captureResult{}, err
		}
		return captureResult{Entry: row, Created: false}, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return captureResult{}, err
	}

	// Create path.
	var seriesID int64
	switch {
	case capture.SeriesID != nil:
		ok, err := s.repo.SeriesExists(ctx, userID, *capture.SeriesID)
		if err != nil {
			return captureResult{}, err
		}
		if !ok {
			return captureResult{}, ErrSeriesNotFound
		}
		seriesID = *capture.SeriesID
	case capture.NewSeriesTitle != nil && *capture.NewSeriesTitle != "":
		id, err := sc.Create(ctx, userID, *capture.NewSeriesTitle)
		if err != nil {
			return captureResult{}, fmt.Errorf("entries: create implicit series: %w", err)
		}
		seriesID = id
	default:
		return captureResult{}, ErrSeriesRequired
	}

	now := time.Now().UTC()
	row, err := s.repo.InsertEntry(ctx, InsertEntryParams{
		UserID:         userID,
		SeriesID:       seriesID,
		SiteHost:       capture.SiteHost,
		SeriesSlug:     capture.SeriesSlug,
		SiteTitle:      capture.SiteTitle,
		LastChapter:    chapter,
		LastURL:        capture.URL,
		LastCapturedAt: now,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		return captureResult{}, err
	}
	return captureResult{Entry: row, Created: true}, nil
}

// ListReadingPositions returns a page of reading-position entries
// plus the total count for the user. Pagination defaults / bounds are
// enforced at the binding layer via the tags on [models.EntryFilter];
// this method assumes Limit / Offset are already valid.
func (s *Service) ListReadingPositions(ctx context.Context, userID int64, filter models.EntryFilter) (models.EntryList, error) {
	if filter.SeriesID != nil {
		rows, err := s.repo.ListEntriesBySeries(ctx, ListEntriesBySeriesParams{
			UserID:   userID,
			SeriesID: *filter.SeriesID,
			Limit:    int64(filter.Limit),
			Offset:   int64(filter.Offset),
		})
		if err != nil {
			return models.EntryList{}, err
		}
		total, err := s.repo.CountEntriesBySeries(ctx, userID, *filter.SeriesID)
		if err != nil {
			return models.EntryList{}, err
		}
		return models.EntryList{Items: rows, Total: total}, nil
	}
	rows, err := s.repo.ListEntriesAll(ctx, ListEntriesAllParams{
		UserID: userID,
		Limit:  int64(filter.Limit),
		Offset: int64(filter.Offset),
	})
	if err != nil {
		return models.EntryList{}, err
	}
	total, err := s.repo.CountEntriesAll(ctx, userID)
	if err != nil {
		return models.EntryList{}, err
	}
	return models.EntryList{Items: rows, Total: total}, nil
}

// ListForSeries returns every entry attached to seriesID. Used by
// series.Service.Detail to embed the per-site breakdown. Off the
// public models.EntriesService interface because it's an
// internal-only cross-service call.
func (s *Service) ListForSeries(ctx context.Context, userID, seriesID int64) ([]models.Entry, error) {
	return s.repo.ListEntriesAllForSeries(ctx, userID, seriesID)
}

// get returns one entry, scoped to the owning user. Unexported — only
// [Service.AdjustReadingPosition] uses it. Kept off the
// [EntriesService] surface so the interface stays domain-verb only.
func (s *Service) get(ctx context.Context, userID, entryID int64) (models.Entry, error) {
	return s.repo.GetEntryByID(ctx, userID, entryID)
}

// AdjustReadingPosition applies a partial update ("adjust this
// position"). Reassignment is just SeriesID being set to a different
// (existing, owned) series. Manual correction is LastChapter /
// LastURL / SiteTitle.
func (s *Service) AdjustReadingPosition(ctx context.Context, userID, entryID int64, patch models.EntryPatch) (models.Entry, error) {
	current, err := s.get(ctx, userID, entryID)
	if err != nil {
		return models.Entry{}, err
	}
	seriesID := current.SeriesID
	if patch.SeriesID != nil && *patch.SeriesID != seriesID {
		ok, err := s.repo.SeriesExists(ctx, userID, *patch.SeriesID)
		if err != nil {
			s.logger.Error("adjust: series exists check",
				zap.Int64("user_id", userID),
				zap.Int64("entry_id", entryID),
				zap.Int64("series_id", *patch.SeriesID),
				zap.Error(err),
			)
			return models.Entry{}, err
		}
		if !ok {
			s.logger.Info("adjust rejected: series not found",
				zap.Int64("user_id", userID),
				zap.Int64("entry_id", entryID),
				zap.Int64("series_id", *patch.SeriesID),
			)
			return models.Entry{}, ErrSeriesNotFound
		}
		seriesID = *patch.SeriesID
	}
	lastChapter := current.LastChapter
	if patch.LastChapter != nil {
		lastChapter = *patch.LastChapter
	}
	lastURL := current.LastURL
	if patch.LastURL != nil {
		lastURL = *patch.LastURL
	}
	siteTitle := current.SiteTitle
	if patch.SiteTitle != nil {
		siteTitle = *patch.SiteTitle
	}
	now := time.Now().UTC()
	row, err := s.repo.UpdateEntry(ctx, UpdateEntryParams{
		ID:          entryID,
		UserID:      userID,
		SeriesID:    seriesID,
		LastChapter: lastChapter,
		LastURL:     lastURL,
		SiteTitle:   siteTitle,
		UpdatedAt:   now,
	})
	if err != nil {
		s.logger.Error("adjust: update entry",
			zap.Int64("user_id", userID),
			zap.Int64("entry_id", entryID),
			zap.Error(err),
		)
		return models.Entry{}, err
	}
	s.logger.Info("reading position adjusted",
		zap.Int64("user_id", userID),
		zap.Int64("entry_id", entryID),
		zap.Int64("series_id", seriesID),
		zap.Float64("last_chapter", lastChapter),
	)
	return row, nil
}

// ForgetReadingPosition removes an entry ("forget where I was on this
// site").
func (s *Service) ForgetReadingPosition(ctx context.Context, userID, entryID int64) error {
	n, err := s.repo.DeleteEntry(ctx, userID, entryID)
	if err != nil {
		s.logger.Error("forget: delete entry",
			zap.Int64("user_id", userID),
			zap.Int64("entry_id", entryID),
			zap.Error(err),
		)
		return err
	}
	if n == 0 {
		s.logger.Info("forget rejected: entry not found",
			zap.Int64("user_id", userID),
			zap.Int64("entry_id", entryID),
		)
		return ErrNotFound
	}
	s.logger.Info("reading position forgotten",
		zap.Int64("user_id", userID),
		zap.Int64("entry_id", entryID),
	)
	return nil
}
