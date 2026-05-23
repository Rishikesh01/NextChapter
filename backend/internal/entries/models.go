package entries

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
)

// captureResult carries the row plus whether it was newly created.
// Package-internal — the public surface returns (entry, created, err)
// via [EntriesService.Capture].
type captureResult struct {
	Entry   models.Entry
	Created bool
}

// getEntryByKeyParams is the input for [repository.getEntryByKey].
type getEntryByKeyParams struct {
	UserID     int64
	SiteHost   string
	SeriesSlug string
}

// insertEntryParams is the input for [repository.insertEntry].
type insertEntryParams struct {
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

// advanceEntryParams is the input for [repository.advanceEntry].
type advanceEntryParams struct {
	ID             int64
	UserID         int64
	LastChapter    float64
	LastURL        string
	SiteTitle      string
	LastCapturedAt time.Time
	UpdatedAt      time.Time
}

// updateEntryParams is the input for [repository.updateEntry].
type updateEntryParams struct {
	ID          int64
	UserID      int64
	SeriesID    int64
	LastChapter float64
	LastURL     string
	SiteTitle   string
	UpdatedAt   time.Time
}

// listEntriesAllParams paginates the user-wide entry list.
type listEntriesAllParams struct {
	UserID int64
	Limit  int64
	Offset int64
}

// listEntriesBySeriesParams paginates the per-series entry list.
type listEntriesBySeriesParams struct {
	UserID   int64
	SeriesID int64
	Limit    int64
	Offset   int64
}

// repository is the persistence surface for the entries domain. The
// service in this package depends on this interface; the concrete
// implementations in repository_sqlite.go and repository_postgres.go
// are the only things in the package that import sqlc-generated code.
type repository interface {
	getEntryByID(ctx context.Context, userID, entryID int64) (models.Entry, error)
	getEntryByKey(ctx context.Context, p getEntryByKeyParams) (models.Entry, error)
	insertEntry(ctx context.Context, p insertEntryParams) (models.Entry, error)
	advanceEntry(ctx context.Context, p advanceEntryParams) (models.Entry, error)
	updateEntry(ctx context.Context, p updateEntryParams) (models.Entry, error)
	deleteEntry(ctx context.Context, userID, entryID int64) (int64, error)
	listEntriesAll(ctx context.Context, p listEntriesAllParams) ([]models.Entry, error)
	listEntriesBySeries(ctx context.Context, p listEntriesBySeriesParams) ([]models.Entry, error)
	listEntriesAllForSeries(ctx context.Context, userID, seriesID int64) ([]models.Entry, error)
	countEntriesAll(ctx context.Context, userID int64) (int64, error)
	countEntriesBySeries(ctx context.Context, userID, seriesID int64) (int64, error)
	seriesExists(ctx context.Context, userID, seriesID int64) (bool, error)
	listTrackedHosts(ctx context.Context, userID int64) ([]string, error)
}
