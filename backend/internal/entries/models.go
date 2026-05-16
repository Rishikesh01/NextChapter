package entries

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// captureResult carries the row plus whether it was newly created.
// Package-internal — the public surface returns (entry, created, err)
// via models.EntriesService.Capture.
type captureResult struct {
	Entry   models.Entry
	Created bool
}

// GetEntryByKeyParams is the input for [Repository.GetEntryByKey].
type GetEntryByKeyParams struct {
	UserID     int64
	SiteHost   string
	SeriesSlug string
}

// InsertEntryParams is the input for [Repository.InsertEntry].
type InsertEntryParams struct {
	UserID         int64
	SeriesID       int64
	SiteHost       string
	SeriesSlug     string
	SiteTitle      string
	LastChapter    float64
	LastURL        string
	LastCapturedAt time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AdvanceEntryParams is the input for [Repository.AdvanceEntry].
type AdvanceEntryParams struct {
	ID             int64
	UserID         int64
	LastChapter    float64
	LastURL        string
	SiteTitle      string
	LastCapturedAt time.Time
	UpdatedAt      time.Time
}

// UpdateEntryParams is the input for [Repository.UpdateEntry].
type UpdateEntryParams struct {
	ID          int64
	UserID      int64
	SeriesID    int64
	LastChapter float64
	LastURL     string
	SiteTitle   string
	UpdatedAt   time.Time
}

// ListEntriesAllParams paginates the user-wide entry list.
type ListEntriesAllParams struct {
	UserID int64
	Limit  int64
	Offset int64
}

// ListEntriesBySeriesParams paginates the per-series entry list.
type ListEntriesBySeriesParams struct {
	UserID   int64
	SeriesID int64
	Limit    int64
	Offset   int64
}

// Repository is the persistence surface for the entries domain. The
// service in this package depends on this interface; the concrete
// implementation in [NewRepository] is the only thing in the package
// that imports the sqlc-generated code.
type Repository interface {
	GetEntryByID(ctx context.Context, userID, id int64) (models.Entry, error)
	GetEntryByKey(ctx context.Context, p GetEntryByKeyParams) (models.Entry, error)
	InsertEntry(ctx context.Context, p InsertEntryParams) (models.Entry, error)
	AdvanceEntry(ctx context.Context, p AdvanceEntryParams) (models.Entry, error)
	UpdateEntry(ctx context.Context, p UpdateEntryParams) (models.Entry, error)
	DeleteEntry(ctx context.Context, userID, id int64) (int64, error)
	ListEntriesAll(ctx context.Context, p ListEntriesAllParams) ([]models.Entry, error)
	ListEntriesBySeries(ctx context.Context, p ListEntriesBySeriesParams) ([]models.Entry, error)
	ListEntriesAllForSeries(ctx context.Context, userID, seriesID int64) ([]models.Entry, error)
	CountEntriesAll(ctx context.Context, userID int64) (int64, error)
	CountEntriesBySeries(ctx context.Context, userID, seriesID int64) (int64, error)
	SeriesExists(ctx context.Context, userID, seriesID int64) (bool, error)
}

// repository is the concrete sqlc-backed implementation of [Repository].
type repository struct {
	q *gen.Queries
}
