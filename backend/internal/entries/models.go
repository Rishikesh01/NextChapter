package entries

import (
	"context"
	"time"

	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// Service exposes the entries domain to handlers.
type Service struct {
	repo Repository
	now  func() time.Time
}

// Entry mirrors the API "Entry" schema. It exists in its own type (vs
// gen.Entry) so handlers can return it directly through encoding/json
// with snake_case field tags.
type Entry struct {
	ID             int64
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

// CaptureParams is both the POST /entries/capture JSON body and the
// input to [Service.Capture]. Chapter is a *float64 so that a missing
// field is distinguishable from chapter 0 (which is a valid value);
// the `required` binding tag rejects the missing case.
type CaptureParams struct {
	SiteHost       string   `json:"site_host"       binding:"required,min=1,max=253"`
	SeriesSlug     string   `json:"series_slug"     binding:"required,min=1,max=512"`
	SiteTitle      string   `json:"site_title"      binding:"required,min=1,max=512"`
	Chapter        *float64 `json:"chapter"         binding:"required,gte=0"`
	URL            string   `json:"url"             binding:"required,min=1,max=2048,url"`
	SeriesID       *int64   `json:"series_id"       binding:"omitempty,min=1"`
	NewSeriesTitle *string  `json:"new_series_title"`
}

// CaptureResult carries the row plus whether it was newly created.
type CaptureResult struct {
	Entry   Entry
	Created bool
}

// ListParams paginates the entries list.
type ListParams struct {
	SeriesID *int64
	Limit    int
	Offset   int
}

// UpdateParams is both the PATCH /entries/{id} JSON body and the
// input to [Service.Update]. Pointer fields use the standard
// absent/present binary: nil means "leave the column alone".
type UpdateParams struct {
	SeriesID    *int64   `json:"series_id,omitempty"    binding:"omitempty,min=1"`
	LastChapter *float64 `json:"last_chapter,omitempty" binding:"omitempty,min=0"`
	LastURL     *string  `json:"last_url,omitempty"`
	SiteTitle   *string  `json:"site_title,omitempty"   binding:"omitempty,min=1,max=512"`
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

// SeriesCreator is implemented by the series service. We accept it as
// an interface so that the entries package does not import series
// (which imports entries for its Detail loader).
type SeriesCreator interface {
	Create(ctx context.Context, userID int64, title string) (int64, error)
}

// Repository is the persistence surface for the entries domain. The
// service in this package depends on this interface; the concrete
// implementation in [NewRepository] is the only thing in the package
// that imports the sqlc-generated code.
type Repository interface {
	GetEntryByID(ctx context.Context, userID, id int64) (Entry, error)
	GetEntryByKey(ctx context.Context, p GetEntryByKeyParams) (Entry, error)
	InsertEntry(ctx context.Context, p InsertEntryParams) (Entry, error)
	AdvanceEntry(ctx context.Context, p AdvanceEntryParams) (Entry, error)
	UpdateEntry(ctx context.Context, p UpdateEntryParams) (Entry, error)
	DeleteEntry(ctx context.Context, userID, id int64) (int64, error)
	ListEntriesAll(ctx context.Context, p ListEntriesAllParams) ([]Entry, error)
	ListEntriesBySeries(ctx context.Context, p ListEntriesBySeriesParams) ([]Entry, error)
	ListEntriesAllForSeries(ctx context.Context, userID, seriesID int64) ([]Entry, error)
	CountEntriesAll(ctx context.Context, userID int64) (int64, error)
	CountEntriesBySeries(ctx context.Context, userID, seriesID int64) (int64, error)
	SeriesExists(ctx context.Context, userID, seriesID int64) (bool, error)
}

// repository is the concrete sqlc-backed implementation of [Repository].
type repository struct {
	q *gen.Queries
}
