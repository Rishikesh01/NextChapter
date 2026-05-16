// Package series owns the series CRUD domain. The "highest_chapter"
// rollup is computed in SQL via a correlated subquery (see series.sql);
// this package's job is to translate the sqlc result types into the
// API-friendly Summary / Detail shapes that the openapi schema speaks.
package series

import (
	"context"
	"errors"
	"time"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/entries"
)

// ErrNotFound is returned when a lookup misses, or the row exists but
// belongs to a different user. Handlers turn this into 404.
var ErrNotFound = errors.New("series: not found")

// ErrInvalidStatus is returned when a caller supplies a status that is
// not in the openapi enum.
var ErrInvalidStatus = errors.New("series: invalid status")

// NewService builds a Service. The entries.Service is used to load the
// per-series entry list inside [Detail].
func NewService(repo Repository, e *entries.Service, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, entries: e, now: now}
}

// Create inserts a new series. Validation lives in the handler; this
// method assumes Title is non-empty.
func (s *Service) Create(ctx context.Context, userID int64, p CreateParams) (Series, error) {
	status := p.Status
	if status == "" {
		status = constants.DefaultSeriesStatus
	}
	if _, ok := validStatuses[status]; !ok {
		return Series{}, ErrInvalidStatus
	}
	now := s.now().UTC()
	return s.repo.InsertSeries(ctx, InsertSeriesParams{
		UserID:    userID,
		Title:     p.Title,
		Status:    status,
		Rating:    p.Rating,
		Notes:     p.Notes,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// Get returns a single Series row for the owning user, or ErrNotFound.
func (s *Service) Get(ctx context.Context, userID, id int64) (Series, error) {
	return s.repo.GetSeriesByID(ctx, userID, id)
}

// List returns a page of summaries (with rollups) plus the total count
// for the user, respecting the optional status filter.
func (s *Service) List(ctx context.Context, userID int64, p ListParams) ([]Summary, int64, error) {
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

	if p.Status != "" {
		if _, ok := validStatuses[p.Status]; !ok {
			return nil, 0, ErrInvalidStatus
		}
		rows, err := s.repo.ListSummariesByStatus(ctx, ListSummariesByStatusParams{
			UserID: userID,
			Status: p.Status,
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, 0, err
		}
		total, err := s.repo.CountByStatus(ctx, userID, p.Status)
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

// Detail returns the summary plus the full per-site entry list.
func (s *Service) Detail(ctx context.Context, userID, id int64) (Detail, error) {
	summary, err := s.repo.GetSummary(ctx, userID, id)
	if err != nil {
		return Detail{}, err
	}
	es, err := s.entries.ListForSeries(ctx, userID, id)
	if err != nil {
		return Detail{}, err
	}
	return Detail{Summary: summary, Entries: es}, nil
}

// Update applies a partial patch to a series. Fields with nil pointers
// are left untouched. Returns ErrNotFound if no row matched.
func (s *Service) Update(ctx context.Context, userID, id int64, p UpdateParams) (Series, error) {
	current, err := s.Get(ctx, userID, id)
	if err != nil {
		return Series{}, err
	}
	title := current.Title
	if p.Title != nil {
		title = *p.Title
	}
	status := current.Status
	if p.Status != nil {
		if _, ok := validStatuses[*p.Status]; !ok {
			return Series{}, ErrInvalidStatus
		}
		status = *p.Status
	}
	rating := current.Rating
	if p.Rating != nil {
		rating = p.Rating
	}
	notes := current.Notes
	if p.Notes != nil {
		notes = *p.Notes
	}
	now := s.now().UTC()
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

// Delete removes a series (cascading to its entries). Returns ErrNotFound
// if no row matched.
func (s *Service) Delete(ctx context.Context, userID, id int64) error {
	n, err := s.repo.DeleteSeries(ctx, userID, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ValidStatus reports whether s is in the SeriesStatus enum.
func ValidStatus(status string) bool {
	_, ok := validStatuses[status]
	return ok
}

// CreateImplicit is the SeriesCreator adapter used by the entries
// service when a capture call asks for a brand-new series via
// new_series_title. It applies defaults (status=reading, no rating, no
// notes) and returns just the new id.
func (s *Service) CreateImplicit(ctx context.Context, userID int64, title string) (int64, error) {
	row, err := s.Create(ctx, userID, CreateParams{Title: title})
	if err != nil {
		return 0, err
	}
	return row.ID, nil
}
