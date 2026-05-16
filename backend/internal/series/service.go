// Package series owns the series CRUD domain. The "highest_chapter"
// rollup is computed in SQL via a correlated subquery (see series.sql);
// this package's job is to translate the sqlc result types into the
// API-friendly Summary / Detail shapes that the openapi schema speaks.
package series

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/entries"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// Service code does not validate, default, or clamp inputs — that
// belongs to the binding layer (see [models.SeriesFilter] for the
// pagination tags). Filter.Limit / Filter.Offset arrive
// already-bounded; this service trusts them and passes them through.

// Re-exported sentinels so callers inside this package keep the
// short name. The canonical values live in [models] so handlers can
// errors.Is without importing this package.
var (
	ErrNotFound      = models.ErrSeriesNotFound
	ErrInvalidStatus = models.ErrSeriesInvalidStatus
)

// SeriesService is the surface the HTTP handlers consume for the
// series CRUD endpoints. Method names are domain verbs qualified by
// the resource noun (TrackSeries / ListTrackedSeries / FindSeries /
// InspectSeries / EditSeries / UntrackSeries) so each declaration is
// self-documenting at the interface, not the call site.
type SeriesService interface {
	TrackSeries(ctx context.Context, userID int64, draft models.SeriesNew) (models.Series, error)
	ListTrackedSeries(ctx context.Context, userID int64, filter models.SeriesFilter) (models.SeriesList, error)
	FindSeries(ctx context.Context, userID, seriesID int64) (models.Series, error)
	InspectSeries(ctx context.Context, userID, seriesID int64) (models.SeriesDetail, error)
	EditSeries(ctx context.Context, userID, seriesID int64, patch models.SeriesPatch) (models.Series, error)
	UntrackSeries(ctx context.Context, userID, seriesID int64) error
}

// Service exposes the series domain to handlers.
type Service struct {
	repo    Repository
	entries *entries.Service
	logger  *zap.Logger
}

// Compile-time check: the concrete Service satisfies the
// SeriesService surface that handlers consume.
var _ SeriesService = (*Service)(nil)

// NewService builds a Service. The entries.Service is used to load
// the per-series entry list inside [InspectSeries]. Passing a nil
// logger is fine for tests; a no-op logger is substituted.
func NewService(repo Repository, e *entries.Service, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{repo: repo, entries: e, logger: logger}
}

// TrackSeries inserts a new series ("start tracking this title").
// Validation lives in the handler; this method assumes Title is
// non-empty.
func (s *Service) TrackSeries(ctx context.Context, userID int64, draft models.SeriesNew) (models.Series, error) {
	status := draft.Status
	if status == "" {
		status = constants.DefaultSeriesStatus
	}
	if _, ok := validStatuses[status]; !ok {
		s.logger.Info("track rejected: invalid status",
			zap.Int64("user_id", userID),
			zap.String("status", status),
		)
		return models.Series{}, ErrInvalidStatus
	}
	now := time.Now().UTC()
	row, err := s.repo.InsertSeries(ctx, InsertSeriesParams{
		UserID:    userID,
		Title:     draft.Title,
		Status:    status,
		Rating:    draft.Rating,
		Notes:     draft.Notes,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		s.logger.Error("track: insert series",
			zap.Int64("user_id", userID),
			zap.String("title", draft.Title),
			zap.Error(err),
		)
		return models.Series{}, err
	}
	s.logger.Info("series tracked",
		zap.Int64("user_id", userID),
		zap.Int64("series_id", row.ID),
		zap.String("status", row.Status),
	)
	return row, nil
}

// FindSeries returns a single Series row for the owning user, or
// ErrNotFound.
func (s *Service) FindSeries(ctx context.Context, userID, seriesID int64) (models.Series, error) {
	return s.repo.GetSeriesByID(ctx, userID, seriesID)
}

// ListTrackedSeries returns a page of summaries (with rollups) plus
// the total count for the user, respecting the optional status filter.
// This is the "user's tracked series" view. Pagination defaults /
// bounds and the status enum are enforced at the binding layer via
// the tags on [models.SeriesFilter]; this method assumes Limit /
// Offset / Status are already valid.
func (s *Service) ListTrackedSeries(ctx context.Context, userID int64, filter models.SeriesFilter) (models.SeriesList, error) {
	s.logger.Debug("list series",
		zap.Int64("user_id", userID),
		zap.String("status", filter.Status),
		zap.Int("limit", filter.Limit),
		zap.Int("offset", filter.Offset),
	)
	if filter.Status != "" {
		rows, err := s.repo.ListSummariesByStatus(ctx, ListSummariesByStatusParams{
			UserID: userID,
			Status: filter.Status,
			Limit:  int64(filter.Limit),
			Offset: int64(filter.Offset),
		})
		if err != nil {
			return models.SeriesList{}, err
		}
		total, err := s.repo.CountByStatus(ctx, userID, filter.Status)
		if err != nil {
			return models.SeriesList{}, err
		}
		return models.SeriesList{Items: rows, Total: total}, nil
	}

	rows, err := s.repo.ListSummariesAll(ctx, ListSummariesAllParams{
		UserID: userID,
		Limit:  int64(filter.Limit),
		Offset: int64(filter.Offset),
	})
	if err != nil {
		return models.SeriesList{}, err
	}
	total, err := s.repo.CountAll(ctx, userID)
	if err != nil {
		return models.SeriesList{}, err
	}
	return models.SeriesList{Items: rows, Total: total}, nil
}

// InspectSeries returns the summary plus the full per-site entry list.
func (s *Service) InspectSeries(ctx context.Context, userID, seriesID int64) (models.SeriesDetail, error) {
	summary, err := s.repo.GetSummary(ctx, userID, seriesID)
	if err != nil {
		return models.SeriesDetail{}, err
	}
	es, err := s.entries.ListForSeries(ctx, userID, seriesID)
	if err != nil {
		return models.SeriesDetail{}, err
	}
	return models.SeriesDetail{SeriesSummary: summary, Entries: es}, nil
}

// EditSeries applies a partial patch to a series. Fields with nil
// pointers are left untouched. Returns ErrNotFound if no row matched.
func (s *Service) EditSeries(ctx context.Context, userID, seriesID int64, patch models.SeriesPatch) (models.Series, error) {
	current, err := s.FindSeries(ctx, userID, seriesID)
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
			s.logger.Info("edit rejected: invalid status",
				zap.Int64("user_id", userID),
				zap.Int64("series_id", seriesID),
				zap.String("status", *patch.Status),
			)
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
	row, err := s.repo.UpdateSeries(ctx, UpdateSeriesParams{
		ID:        seriesID,
		UserID:    userID,
		Title:     title,
		Status:    status,
		Rating:    rating,
		Notes:     notes,
		UpdatedAt: now,
	})
	if err != nil {
		s.logger.Error("edit: update series",
			zap.Int64("user_id", userID),
			zap.Int64("series_id", seriesID),
			zap.Error(err),
		)
		return models.Series{}, err
	}
	s.logger.Info("series edited",
		zap.Int64("user_id", userID),
		zap.Int64("series_id", seriesID),
		zap.String("status", row.Status),
	)
	return row, nil
}

// UntrackSeries removes a series (cascading to its entries). Returns
// ErrNotFound if no row matched.
func (s *Service) UntrackSeries(ctx context.Context, userID, seriesID int64) error {
	n, err := s.repo.DeleteSeries(ctx, userID, seriesID)
	if err != nil {
		s.logger.Error("untrack: delete series",
			zap.Int64("user_id", userID),
			zap.Int64("series_id", seriesID),
			zap.Error(err),
		)
		return err
	}
	if n == 0 {
		s.logger.Info("untrack rejected: series not found",
			zap.Int64("user_id", userID),
			zap.Int64("series_id", seriesID),
		)
		return ErrNotFound
	}
	s.logger.Info("series untracked",
		zap.Int64("user_id", userID),
		zap.Int64("series_id", seriesID),
	)
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
	row, err := s.TrackSeries(ctx, userID, models.SeriesNew{Title: title})
	if err != nil {
		return 0, err
	}
	return row.ID, nil
}
