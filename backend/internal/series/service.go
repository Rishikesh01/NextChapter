// Package series owns the series CRUD domain. The "highest_chapter"
// rollup is computed in SQL via a correlated subquery (see series.sql);
// this package's job is to translate the sqlc result types into the
// API-friendly Summary / Detail shapes that the openapi schema speaks.
package series

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/entries"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// Re-exported sentinels so callers inside this package keep the
// short name. The canonical values live in [models] so handlers can
// errors.Is without importing this package.
var (
	ErrNotFound      = models.ErrSeriesNotFound
	ErrInvalidStatus = models.ErrSeriesInvalidStatus
)

// SeriesService is the surface the HTTP handlers consume for the
// series CRUD endpoints. Method names are domain verbs (Track / Library /
// Find / Inspect / Edit / Untrack) rather than generic CRUD so they read
// like user actions in the product's "where was I" framing.
type SeriesService interface {
	Track(ctx context.Context, userID int64, draft models.SeriesNew) (models.Series, error)
	Library(ctx context.Context, userID int64, filter models.SeriesFilter) ([]models.SeriesSummary, int64, error)
	Find(ctx context.Context, userID, id int64) (models.Series, error)
	Inspect(ctx context.Context, userID, id int64) (models.SeriesDetail, error)
	Edit(ctx context.Context, userID, id int64, patch models.SeriesPatch) (models.Series, error)
	Untrack(ctx context.Context, userID, id int64) error
}

// Service exposes the series domain to handlers.
type Service struct {
	repo    Repository
	entries *entries.Service
}

// Compile-time check: the concrete Service satisfies the
// SeriesService surface that handlers consume.
var _ SeriesService = (*Service)(nil)

// NewService builds a Service. The entries.Service is used to load
// the per-series entry list inside [Detail].
func NewService(repo Repository, e *entries.Service) *Service {
	return &Service{repo: repo, entries: e}
}

// Track inserts a new series ("start tracking this title"). Validation
// lives in the handler; this method assumes Title is non-empty.
func (s *Service) Track(ctx context.Context, userID int64, draft models.SeriesNew) (models.Series, error) {
	status := draft.Status
	if status == "" {
		status = constants.DefaultSeriesStatus
	}
	if _, ok := validStatuses[status]; !ok {
		return models.Series{}, ErrInvalidStatus
	}
	now := time.Now().UTC()
	return s.repo.InsertSeries(ctx, InsertSeriesParams{
		UserID:    userID,
		Title:     draft.Title,
		Status:    status,
		Rating:    draft.Rating,
		Notes:     draft.Notes,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// Find returns a single Series row for the owning user, or ErrNotFound.
func (s *Service) Find(ctx context.Context, userID, id int64) (models.Series, error) {
	return s.repo.GetSeriesByID(ctx, userID, id)
}

// Library returns a page of summaries (with rollups) plus the total
// count for the user, respecting the optional status filter. This is
// the "user's tracked series" view.
func (s *Service) Library(ctx context.Context, userID int64, filter models.SeriesFilter) ([]models.SeriesSummary, int64, error) {
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

	if filter.Status != "" {
		if _, ok := validStatuses[filter.Status]; !ok {
			return nil, 0, ErrInvalidStatus
		}
		rows, err := s.repo.ListSummariesByStatus(ctx, ListSummariesByStatusParams{
			UserID: userID,
			Status: filter.Status,
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, 0, err
		}
		total, err := s.repo.CountByStatus(ctx, userID, filter.Status)
		if err != nil {
			return nil, 0, err
		}
		return rows, total, nil
	}

	rows, err := s.repo.ListSummariesAll(ctx, ListSummariesAllParams{
		UserID: userID,
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountAll(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// Inspect returns the summary plus the full per-site entry list.
func (s *Service) Inspect(ctx context.Context, userID, id int64) (models.SeriesDetail, error) {
	summary, err := s.repo.GetSummary(ctx, userID, id)
	if err != nil {
		return models.SeriesDetail{}, err
	}
	es, err := s.entries.ListForSeries(ctx, userID, id)
	if err != nil {
		return models.SeriesDetail{}, err
	}
	return models.SeriesDetail{SeriesSummary: summary, Entries: es}, nil
}

// Edit applies a partial patch to a series. Fields with nil
// pointers are left untouched. Returns ErrNotFound if no row matched.
func (s *Service) Edit(ctx context.Context, userID, id int64, patch models.SeriesPatch) (models.Series, error) {
	current, err := s.Find(ctx, userID, id)
	if err != nil {
		return models.Series{}, err
	}
	title := current.Title
	if patch.Title != nil {
		title = *patch.Title
	}
	status := current.Status
	if patch.Status != nil {
		if _, ok := validStatuses[*patch.Status]; !ok {
			return models.Series{}, ErrInvalidStatus
		}
		status = *patch.Status
	}
	rating := current.Rating
	if patch.Rating != nil {
		rating = patch.Rating
	}
	notes := current.Notes
	if patch.Notes != nil {
		notes = *patch.Notes
	}
	now := time.Now().UTC()
	return s.repo.UpdateSeries(ctx, UpdateSeriesParams{
		ID:        id,
		UserID:    userID,
		Title:     title,
		Status:    status,
		Rating:    rating,
		Notes:     notes,
		UpdatedAt: now,
	})
}

// Untrack removes a series (cascading to its entries). Returns
// ErrNotFound if no row matched.
func (s *Service) Untrack(ctx context.Context, userID, id int64) error {
	n, err := s.repo.DeleteSeries(ctx, userID, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ValidStatus reports whether status is in the SeriesStatus enum.
func ValidStatus(status string) bool {
	_, ok := validStatuses[status]
	return ok
}

// CreateImplicit is the models.SeriesCreator adapter used by the
// entries service when a capture call asks for a brand-new series via
// new_series_title. It applies defaults (status=reading, no rating,
// no notes) and returns just the new id.
func (s *Service) CreateImplicit(ctx context.Context, userID int64, title string) (int64, error) {
	row, err := s.Track(ctx, userID, models.SeriesNew{Title: title})
	if err != nil {
		return 0, err
	}
	return row.ID, nil
}
