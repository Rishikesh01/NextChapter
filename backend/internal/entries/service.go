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

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// EntriesService is the surface the HTTP handlers consume for the
// entries endpoints. Capture returns (entry, created, err) — the bool
// distinguishes the 201 vs 200 paths on the wire. Method names are
// domain verbs (Capture / Positions / Adjust / Forget) so they read
// like reading-position actions rather than generic CRUD.
type EntriesService interface {
	Capture(ctx context.Context, userID int64, capture models.EntryCapture, sc models.SeriesCreator) (models.Entry, bool, error)
	Positions(ctx context.Context, userID int64, filter models.EntryFilter) ([]models.Entry, int64, error)
	Adjust(ctx context.Context, userID, entryID int64, patch models.EntryPatch) (models.Entry, error)
	Forget(ctx context.Context, userID, entryID int64) error
}

// Service exposes the entries domain to handlers.
type Service struct {
	repo Repository
}

// Compile-time check: the concrete Service satisfies the
// EntriesService surface that handlers consume.
var _ EntriesService = (*Service)(nil)

// NewService builds a Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Capture implements POST /entries/capture per the openapi spec:
//   - If a row exists for (user, host, slug): advance last_chapter
//     (monotonic — never rewinds) and update last_url / site_title /
//     last_captured_at. 200 OK, idempotent on equal chapter.
//   - Else: create a new row attached to either p.SeriesID (must exist
//     and belong to the user) or a fresh series titled
//     *p.NewSeriesTitle. 201 Created.
//
// The bool return is "was-created": true => 201, false => 200.
func (s *Service) Capture(ctx context.Context, userID int64, capture models.EntryCapture, sc models.SeriesCreator) (models.Entry, bool, error) {
	res, err := s.capture(ctx, userID, capture, sc)
	if err != nil {
		return models.Entry{}, false, err
	}
	return res.Entry, res.Created, nil
}

// capture is the unwrapped core; it returns the package-internal
// captureResult and is kept separate from [Service.Capture] for
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

// Positions returns a page of reading-position entries plus the total
// count for the user.
func (s *Service) Positions(ctx context.Context, userID int64, filter models.EntryFilter) ([]models.Entry, int64, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = constants.ListLimitDefault
	}
	if limit > constants.ListLimitMax {
		limit = constants.ListLimitMax
	}
	offset := filter.Offset
	if offset < constants.ListOffsetMin {
		offset = constants.ListOffsetMin
	}
	if filter.SeriesID != nil {
		rows, err := s.repo.ListEntriesBySeries(ctx, ListEntriesBySeriesParams{
			UserID:   userID,
			SeriesID: *filter.SeriesID,
			Limit:    int64(limit),
			Offset:   int64(offset),
		})
		if err != nil {
			return nil, 0, err
		}
		total, err := s.repo.CountEntriesBySeries(ctx, userID, *filter.SeriesID)
		if err != nil {
			return nil, 0, err
		}
		return rows, total, nil
	}
	rows, err := s.repo.ListEntriesAll(ctx, ListEntriesAllParams{
		UserID: userID,
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountEntriesAll(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// ListForSeries returns every entry attached to seriesID. Used by
// series.Service.Detail to embed the per-site breakdown. Off the
// public models.EntriesService interface because it's an
// internal-only cross-service call.
func (s *Service) ListForSeries(ctx context.Context, userID, seriesID int64) ([]models.Entry, error) {
	return s.repo.ListEntriesAllForSeries(ctx, userID, seriesID)
}

// get returns one entry, scoped to the owning user. Unexported — only
// [Service.Adjust] uses it. Kept off the [EntriesService] surface so
// the interface stays domain-verb only.
func (s *Service) get(ctx context.Context, userID, id int64) (models.Entry, error) {
	return s.repo.GetEntryByID(ctx, userID, id)
}

// Adjust applies a partial update ("adjust this position"). Reassignment
// is just SeriesID being set to a different (existing, owned) series.
// Manual correction is LastChapter / LastURL / SiteTitle.
func (s *Service) Adjust(ctx context.Context, userID, id int64, patch models.EntryPatch) (models.Entry, error) {
	current, err := s.get(ctx, userID, id)
	if err != nil {
		return models.Entry{}, err
	}
	seriesID := current.SeriesID
	if patch.SeriesID != nil && *patch.SeriesID != seriesID {
		ok, err := s.repo.SeriesExists(ctx, userID, *patch.SeriesID)
		if err != nil {
			return models.Entry{}, err
		}
		if !ok {
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
	return s.repo.UpdateEntry(ctx, UpdateEntryParams{
		ID:          id,
		UserID:      userID,
		SeriesID:    seriesID,
		LastChapter: lastChapter,
		LastURL:     lastURL,
		SiteTitle:   siteTitle,
		UpdatedAt:   now,
	})
}

// Forget removes an entry ("forget where I was on this site").
func (s *Service) Forget(ctx context.Context, userID, id int64) error {
	n, err := s.repo.DeleteEntry(ctx, userID, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
