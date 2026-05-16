package series

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/entries"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// validStatuses is the set built from [constants.AllSeriesStatuses] for
// O(1) lookup during validation. Mirrors the CHECK constraint on
// series.status.
var validStatuses = func() map[string]struct{} {
	out := make(map[string]struct{}, len(constants.AllSeriesStatuses))
	for _, s := range constants.AllSeriesStatuses {
		out[s] = struct{}{}
	}
	return out
}()

// Service exposes the series domain to handlers.
type Service struct {
	repo    Repository
	entries *entries.Service
	now     func() time.Time
}

// Series is the domain shape of a series row. Used by Create / Update /
// Get; the summary queries return [Summary] which carries the rollups
// on top.
type Series struct {
	ID        int64
	UserID    int64
	Title     string
	Status    string
	Rating    *int
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Summary is the listing shape returned by GET /series.
type Summary struct {
	ID             int64
	UserID         int64
	Title          string
	Status         string
	Rating         *int
	Notes          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	HighestChapter *float64
	EntryCount     int64
	LastCapturedAt *time.Time
}

// Detail is the SeriesDetail shape returned by GET /series/{id}.
type Detail struct {
	Summary
	Entries []entries.Entry
}

// CreateParams is both the POST /series JSON body and the input to
// [Service.Create]. Field bounds duplicate the numbers in
// [constants.SeriesTitleMin] / [constants.SeriesTitleMax] /
// [constants.RatingMin] / [constants.RatingMax] /
// [constants.SeriesNotesMax] because Go struct tags can't reference
// constants. Update both when the bounds change.
type CreateParams struct {
	Title  string `json:"title"            binding:"required,min=1,max=256"`
	Status string `json:"status,omitempty" binding:"omitempty,oneof=reading completed on_hold dropped plan_to_read"`
	Rating *int   `json:"rating,omitempty" binding:"omitempty,min=1,max=10"`
	Notes  string `json:"notes,omitempty"  binding:"max=8192"`
}

// ListParams configures a paginated list of summaries.
type ListParams struct {
	Status string // optional; "" = all statuses
	Limit  int
	Offset int
}

// UpdateParams is both the PATCH /series/{id} JSON body and the input
// to [Service.Update]. Each pointer field is the standard absent/present
// binary: nil means "leave the column alone". The v1 API does not
// support *clearing* the rating column via PATCH — `rating: null` on the
// wire is treated the same as the field being absent. If clearing
// becomes a product requirement it will be a separate endpoint, not a
// side-effect of PATCH. Bounds mirror [CreateParams].
type UpdateParams struct {
	Title  *string `json:"title,omitempty"  binding:"omitempty,min=1,max=256"`
	Status *string `json:"status,omitempty" binding:"omitempty,oneof=reading completed on_hold dropped plan_to_read"`
	Rating *int    `json:"rating,omitempty" binding:"omitempty,min=1,max=10"`
	Notes  *string `json:"notes,omitempty"  binding:"omitempty,max=8192"`
}

// InsertSeriesParams is the input for [Repository.InsertSeries].
type InsertSeriesParams struct {
	UserID    int64
	Title     string
	Status    string
	Rating    *int
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UpdateSeriesParams is the input for [Repository.UpdateSeries].
type UpdateSeriesParams struct {
	ID        int64
	UserID    int64
	Title     string
	Status    string
	Rating    *int
	Notes     string
	UpdatedAt time.Time
}

// ListSummariesAllParams paginates the user-wide series rollup list.
type ListSummariesAllParams struct {
	UserID int64
	Limit  int64
	Offset int64
}

// ListSummariesByStatusParams paginates the status-filtered series
// rollup list.
type ListSummariesByStatusParams struct {
	UserID int64
	Status string
	Limit  int64
	Offset int64
}

// Repository is the persistence surface for the series domain. The
// service in this package depends on this interface; the concrete
// implementation in [NewRepository] is the only thing in the package
// that imports the sqlc-generated code.
//
// Note: the rollup queries (ListAll / ListByStatus / GetSummary) return
// [Summary] values — the conversion from sqlc's interface{} columns to
// the *float64 / *time.Time domain shape happens *inside* the
// repository, not at the boundary.
type Repository interface {
	InsertSeries(ctx context.Context, p InsertSeriesParams) (Series, error)
	GetSeriesByID(ctx context.Context, userID, id int64) (Series, error)
	UpdateSeries(ctx context.Context, p UpdateSeriesParams) (Series, error)
	DeleteSeries(ctx context.Context, userID, id int64) (int64, error)

	ListSummariesAll(ctx context.Context, p ListSummariesAllParams) ([]Summary, error)
	ListSummariesByStatus(ctx context.Context, p ListSummariesByStatusParams) ([]Summary, error)
	GetSummary(ctx context.Context, userID, id int64) (Summary, error)
	CountAll(ctx context.Context, userID int64) (int64, error)
	CountByStatus(ctx context.Context, userID int64, status string) (int64, error)
}

// repository is the concrete sqlc-backed implementation of [Repository].
type repository struct {
	q *gen.Queries
}
