package series

import (
	"context"
	"time"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// validStatuses backs the O(1) status check; mirrors the CHECK
// constraint on series.status.
var validStatuses = map[string]struct{}{
	constants.StatusReading:    {},
	constants.StatusCompleted:  {},
	constants.StatusOnHold:     {},
	constants.StatusDropped:    {},
	constants.StatusPlanToRead: {},
}

// insertSeriesParams is the input for [repository.insertSeries].
type insertSeriesParams struct {
	UserID    int64
	Title     string
	Status    string
	Rating    *int
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// updateSeriesParams is the input for [repository.updateSeries].
type updateSeriesParams struct {
	ID        int64
	UserID    int64
	Title     string
	Status    string
	Rating    *int
	Notes     string
	UpdatedAt time.Time
}

// listSummariesAllParams paginates the user-wide series rollup list.
// Tags is the optional AND-semantic tag filter; empty means "no tag
// filter". The repository dispatches to a different generated query
// when len(Tags) > 0 — sqlc doesn't do conditional WHERE clauses well,
// so the no-tag and with-tag forms live in separate sqlc queries.
type listSummariesAllParams struct {
	UserID int64
	Limit  int64
	Offset int64
	Tags   []string
}

// listSummariesByStatusParams paginates the status-filtered series
// rollup list. See [listSummariesAllParams] for the Tags semantics.
type listSummariesByStatusParams struct {
	UserID int64
	Status string
	Limit  int64
	Offset int64
	Tags   []string
}

// countAllParams scopes [repository.countAll] with the optional tag
// filter so the listing's `total` matches the filtered set.
type countAllParams struct {
	UserID int64
	Tags   []string
}

// countByStatusParams scopes [repository.countByStatus] with the
// optional tag filter.
type countByStatusParams struct {
	UserID int64
	Status string
	Tags   []string
}

// repository is the persistence surface for the series domain. The
// service in this package depends on this interface; the concrete
// implementations in repository_sqlite.go and repository_postgres.go
// are the only things in the package that import sqlc-generated code.
//
// Note: the rollup queries (listSummariesAll / listSummariesByStatus /
// getSummary) return [models.SeriesSummary] values — the conversion
// from sqlc's interface{} columns to the *float64 / *time.Time domain
// shape happens inside the repository, not at the boundary.
type repository interface {
	insertSeries(ctx context.Context, p insertSeriesParams) (models.Series, error)
	getSeriesByID(ctx context.Context, userID, seriesID int64) (models.Series, error)
	updateSeries(ctx context.Context, p updateSeriesParams) (models.Series, error)
	deleteSeries(ctx context.Context, userID, seriesID int64) (int64, error)

	listSummariesAll(ctx context.Context, p listSummariesAllParams) ([]models.SeriesSummary, error)
	listSummariesByStatus(ctx context.Context, p listSummariesByStatusParams) ([]models.SeriesSummary, error)
	getSummary(ctx context.Context, userID, seriesID int64) (models.SeriesSummary, error)
	countAll(ctx context.Context, p countAllParams) (int64, error)
	countByStatus(ctx context.Context, p countByStatusParams) (int64, error)

	// Tag-related operations. See migration 000005_tags.sql for the
	// table layout. setSeriesTags is a transactional full-replace —
	// every supplied name is upserted into `tag` (per-user) and the
	// `series_tag` rows for the series are rebuilt from scratch. An
	// empty `names` slice clears the series' tags.
	setSeriesTags(ctx context.Context, userID, seriesID int64, names []string) error
	getSeriesTags(ctx context.Context, seriesID int64) ([]string, error)
	listSeriesTagsBatch(ctx context.Context, seriesIDs []int64) (map[int64][]string, error)
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
