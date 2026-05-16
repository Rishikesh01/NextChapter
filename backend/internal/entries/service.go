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
)

// NewService builds a Service.
func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, now: now}
}

// Capture implements POST /entries/capture per the openapi spec:
//   - If a row exists for (user, host, slug): advance last_chapter
//     (monotonic — never rewinds) and update last_url / site_title /
//     last_captured_at. 200 OK, idempotent on equal chapter.
//   - Else: create a new row attached to either p.SeriesID (must exist
//     and belong to the user) or a fresh series titled
//     *p.NewSeriesTitle. 201 Created.
func (s *Service) Capture(ctx context.Context, userID int64, p CaptureParams, sc SeriesCreator) (CaptureResult, error) {
	// p.Chapter is *float64 because the wire treats 0 as a valid chapter
	// (binding:"required" rejects the missing case). Once we're past
	// ShouldBindJSON it's safe to deref.
	chapter := *p.Chapter
	existing, err := s.repo.GetEntryByKey(ctx, GetEntryByKeyParams{
		UserID:     userID,
		SiteHost:   p.SiteHost,
		SeriesSlug: p.SeriesSlug,
	})
	if err == nil {
		// Advance path. Monotonic on last_chapter.
		now := s.now().UTC()
		newChapter := existing.LastChapter
		newURL := existing.LastURL
		newTitle := existing.SiteTitle
		if chapter >= existing.LastChapter {
			newChapter = chapter
			newURL = p.URL
			newTitle = p.SiteTitle
		}
		// Even a no-op click bumps last_captured_at, per "where was I"
		// product framing: the user clicked again so they're reading
		// it again right now.
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
			return CaptureResult{}, err
		}
		return CaptureResult{Entry: row, Created: false}, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return CaptureResult{}, err
	}

	// Create path.
	var seriesID int64
	switch {
	case p.SeriesID != nil:
		ok, err := s.repo.SeriesExists(ctx, userID, *p.SeriesID)
		if err != nil {
			return CaptureResult{}, err
		}
		if !ok {
			return CaptureResult{}, ErrSeriesNotFound
		}
		seriesID = *p.SeriesID
	case p.NewSeriesTitle != nil && *p.NewSeriesTitle != "":
		id, err := sc.Create(ctx, userID, *p.NewSeriesTitle)
		if err != nil {
			return CaptureResult{}, fmt.Errorf("entries: create implicit series: %w", err)
		}
		seriesID = id
	default:
		return CaptureResult{}, ErrSeriesRequired
	}

	now := s.now().UTC()
	row, err := s.repo.InsertEntry(ctx, InsertEntryParams{
		UserID:         userID,
		SeriesID:       seriesID,
		SiteHost:       p.SiteHost,
		SeriesSlug:     p.SeriesSlug,
		SiteTitle:      p.SiteTitle,
		LastChapter:    chapter,
		LastURL:        p.URL,
		LastCapturedAt: now,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		return CaptureResult{}, err
	}
	return CaptureResult{Entry: row, Created: true}, nil
}

// List returns a page of entries plus the total count for the user.
func (s *Service) List(ctx context.Context, userID int64, p ListParams) ([]Entry, int64, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = constants.ListLimitDefault
	}
	if limit > constants.ListLimitMax {
		limit = constants.ListLimitMax
	}
	offset := p.Offset
	if offset < constants.ListOffsetMin {
		offset = constants.ListOffsetMin
	}
	if p.SeriesID != nil {
		rows, err := s.repo.ListEntriesBySeries(ctx, ListEntriesBySeriesParams{
			UserID:   userID,
			SeriesID: *p.SeriesID,
			Limit:    int64(limit),
			Offset:   int64(offset),
		})
		if err != nil {
			return nil, 0, err
		}
		total, err := s.repo.CountEntriesBySeries(ctx, userID, *p.SeriesID)
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
// series.Service.Detail to embed the per-site breakdown.
func (s *Service) ListForSeries(ctx context.Context, userID, seriesID int64) ([]Entry, error) {
	return s.repo.ListEntriesAllForSeries(ctx, userID, seriesID)
}

// Get returns one entry, scoped to the owning user.
func (s *Service) Get(ctx context.Context, userID, id int64) (Entry, error) {
	return s.repo.GetEntryByID(ctx, userID, id)
}

// Update applies a partial patch. Reassignment is just SeriesID being
// set to a different (existing, owned) series. Manual correction is
// LastChapter / LastURL / SiteTitle.
func (s *Service) Update(ctx context.Context, userID, id int64, p UpdateParams) (Entry, error) {
	current, err := s.Get(ctx, userID, id)
	if err != nil {
		return Entry{}, err
	}
	seriesID := current.SeriesID
	if p.SeriesID != nil && *p.SeriesID != seriesID {
		ok, err := s.repo.SeriesExists(ctx, userID, *p.SeriesID)
		if err != nil {
			return Entry{}, err
		}
		if !ok {
			return Entry{}, ErrSeriesNotFound
		}
		seriesID = *p.SeriesID
	}
	lastChapter := current.LastChapter
	if p.LastChapter != nil {
		lastChapter = *p.LastChapter
	}
	lastURL := current.LastURL
	if p.LastURL != nil {
		lastURL = *p.LastURL
	}
	siteTitle := current.SiteTitle
	if p.SiteTitle != nil {
		siteTitle = *p.SiteTitle
	}
	now := s.now().UTC()
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

// Delete removes an entry.
func (s *Service) Delete(ctx context.Context, userID, id int64) error {
	n, err := s.repo.DeleteEntry(ctx, userID, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
