package series

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// validStatuses is the set built from [constants.AllSeriesStatuses]
// for O(1) lookup during validation. Mirrors the CHECK constraint on
// series.status.
var validStatuses = func() map[string]struct{} {
	out := make(map[string]struct{}, len(constants.AllSeriesStatuses))
	for _, s := range constants.AllSeriesStatuses {
		out[s] = struct{}{}
	}
	return out
}()

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
// Tags is the optional AND-semantic tag filter; empty means "no tag
// filter". The repository dispatches to a different generated query
// when len(Tags) > 0 — sqlc doesn't do conditional WHERE clauses well,
// so the no-tag and with-tag forms live in separate sqlc queries.
type ListSummariesAllParams struct {
	UserID int64
	Limit  int64
	Offset int64
	Tags   []string
}

// ListSummariesByStatusParams paginates the status-filtered series
// rollup list. See [ListSummariesAllParams] for the Tags semantics.
type ListSummariesByStatusParams struct {
	UserID int64
	Status string
	Limit  int64
	Offset int64
	Tags   []string
}

// CountAllParams scopes [Repository.CountAll] with the optional tag
// filter so the listing's `total` matches the filtered set.
type CountAllParams struct {
	UserID int64
	Tags   []string
}

// CountByStatusParams scopes [Repository.CountByStatus] with the
// optional tag filter.
type CountByStatusParams struct {
	UserID int64
	Status string
	Tags   []string
}

// Repository is the persistence surface for the series domain. The
// service in this package depends on this interface; the concrete
// implementations in repository_sqlite.go and repository_postgres.go
// are the only things in the package that import sqlc-generated code.
//
// Note: the rollup queries (ListSummariesAll / ListSummariesByStatus /
// GetSummary) return [models.SeriesSummary] values — the conversion
// from sqlc's interface{} columns to the *float64 / *time.Time domain
// shape happens inside the repository, not at the boundary.
type Repository interface {
	InsertSeries(ctx context.Context, p InsertSeriesParams) (models.Series, error)
	GetSeriesByID(ctx context.Context, userID, seriesID int64) (models.Series, error)
	UpdateSeries(ctx context.Context, p UpdateSeriesParams) (models.Series, error)
	DeleteSeries(ctx context.Context, userID, seriesID int64) (int64, error)

	ListSummariesAll(ctx context.Context, p ListSummariesAllParams) ([]models.SeriesSummary, error)
	ListSummariesByStatus(ctx context.Context, p ListSummariesByStatusParams) ([]models.SeriesSummary, error)
	GetSummary(ctx context.Context, userID, seriesID int64) (models.SeriesSummary, error)
	CountAll(ctx context.Context, p CountAllParams) (int64, error)
	CountByStatus(ctx context.Context, p CountByStatusParams) (int64, error)

	// Tag-related operations. See migration 000005_tags.sql for the
	// table layout. SetSeriesTags is a transactional full-replace —
	// every supplied name is upserted into `tag` (per-user) and the
	// `series_tag` rows for the series are rebuilt from scratch. An
	// empty `names` slice clears the series' tags.
	SetSeriesTags(ctx context.Context, userID, seriesID int64, names []string) error
	GetSeriesTags(ctx context.Context, seriesID int64) ([]string, error)
	ListSeriesTagsBatch(ctx context.Context, seriesIDs []int64) (map[int64][]string, error)
}

// listFilterArgs carries the inputs for the hand-rolled tag-filtered
// queries below. The tag-filter queries can't live in sqlc because
// sqlc's slice-expansion path mixes positional / unnumbered placeholders
// in ways that don't translate cleanly between the two engines.
// Building the IN-list inline keeps the placeholder strategy uniform
// per engine.
type listFilterArgs struct {
	userID   int64
	status   string // empty means "no status filter"
	tagNames []string
	limit    int64
	offset   int64
}
