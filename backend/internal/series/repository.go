package series

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// NewRepository builds the concrete Repository backed by a *gen.Queries
// and the underlying *sql.DB. The DB handle is held so [SetSeriesTags]
// can open a transaction; every other method routes through q.
func NewRepository(db *sql.DB, q *gen.Queries) Repository {
	return &repository{db: db, q: q}
}

func (r *repository) InsertSeries(ctx context.Context, p InsertSeriesParams) (models.Series, error) {
	row, err := r.q.CreateSeries(ctx, gen.CreateSeriesParams{
		UserID:    p.UserID,
		Title:     p.Title,
		Status:    p.Status,
		Rating:    intPtrToNullInt64(p.Rating),
		Notes:     p.Notes,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	})
	if err != nil {
		return models.Series{}, fmt.Errorf("series: insert: %w", err)
	}
	return seriesFromGen(row), nil
}

func (r *repository) GetSeriesByID(ctx context.Context, userID, id int64) (models.Series, error) {
	row, err := r.q.GetSeriesByID(ctx, gen.GetSeriesByIDParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Series{}, ErrNotFound
		}
		return models.Series{}, fmt.Errorf("series: get by id: %w", err)
	}
	return seriesFromGen(row), nil
}

func (r *repository) UpdateSeries(ctx context.Context, p UpdateSeriesParams) (models.Series, error) {
	row, err := r.q.UpdateSeries(ctx, gen.UpdateSeriesParams{
		Title:     p.Title,
		Status:    p.Status,
		Rating:    intPtrToNullInt64(p.Rating),
		Notes:     p.Notes,
		UpdatedAt: p.UpdatedAt,
		ID:        p.ID,
		UserID:    p.UserID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Series{}, ErrNotFound
		}
		return models.Series{}, fmt.Errorf("series: update: %w", err)
	}
	return seriesFromGen(row), nil
}

func (r *repository) DeleteSeries(ctx context.Context, userID, id int64) (int64, error) {
	n, err := r.q.DeleteSeries(ctx, gen.DeleteSeriesParams{ID: id, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("series: delete: %w", err)
	}
	return n, nil
}

func (r *repository) ListSummariesAll(ctx context.Context, p ListSummariesAllParams) ([]models.SeriesSummary, error) {
	if len(p.Tags) > 0 {
		return r.listSummariesFilteredByTags(ctx, listFilterArgs{
			userID:   p.UserID,
			tagNames: p.Tags,
			limit:    p.Limit,
			offset:   p.Offset,
		})
	}
	rows, err := r.q.ListSeriesAll(ctx, gen.ListSeriesAllParams{
		UserID: p.UserID,
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("series: list all: %w", err)
	}
	out := make([]models.SeriesSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, summaryFromAllRow(row))
	}
	return out, nil
}

func (r *repository) ListSummariesByStatus(ctx context.Context, p ListSummariesByStatusParams) ([]models.SeriesSummary, error) {
	if len(p.Tags) > 0 {
		return r.listSummariesFilteredByTags(ctx, listFilterArgs{
			userID:   p.UserID,
			status:   p.Status,
			tagNames: p.Tags,
			limit:    p.Limit,
			offset:   p.Offset,
		})
	}
	rows, err := r.q.ListSeriesByStatus(ctx, gen.ListSeriesByStatusParams{
		UserID: p.UserID,
		Status: p.Status,
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("series: list by status: %w", err)
	}
	out := make([]models.SeriesSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, summaryFromStatusRow(row))
	}
	return out, nil
}

func (r *repository) GetSummary(ctx context.Context, userID, id int64) (models.SeriesSummary, error) {
	row, err := r.q.GetSeriesSummary(ctx, gen.GetSeriesSummaryParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.SeriesSummary{}, ErrNotFound
		}
		return models.SeriesSummary{}, fmt.Errorf("series: get summary: %w", err)
	}
	return summaryFromSummaryRow(row), nil
}

func (r *repository) CountAll(ctx context.Context, p CountAllParams) (int64, error) {
	if len(p.Tags) > 0 {
		return r.countFilteredByTags(ctx, listFilterArgs{
			userID:   p.UserID,
			tagNames: p.Tags,
		})
	}
	n, err := r.q.CountSeriesAll(ctx, p.UserID)
	if err != nil {
		return 0, fmt.Errorf("series: count all: %w", err)
	}
	return n, nil
}

func (r *repository) CountByStatus(ctx context.Context, p CountByStatusParams) (int64, error) {
	if len(p.Tags) > 0 {
		return r.countFilteredByTags(ctx, listFilterArgs{
			userID:   p.UserID,
			status:   p.Status,
			tagNames: p.Tags,
		})
	}
	n, err := r.q.CountSeriesByStatus(ctx, gen.CountSeriesByStatusParams{UserID: p.UserID, Status: p.Status})
	if err != nil {
		return 0, fmt.Errorf("series: count by status: %w", err)
	}
	return n, nil
}

// SetSeriesTags is a transactional full-replace of the supplied
// series' tag links. Each name is upserted into `tag` (idempotent via
// UNIQUE(user_id, name)) and the existing `series_tag` rows for this
// series are dropped before the new links are inserted. An empty
// `names` slice clears the series' tags. The transaction guarantees a
// partial failure leaves no half-applied state.
//
// Caller is responsible for de-duplicating names; the repository
// trusts the slice it receives (the service de-dupes after binding
// per the project's "services don't normalise" rule applied to
// repository inputs).
func (r *repository) SetSeriesTags(ctx context.Context, userID, seriesID int64, names []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("series: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	qtx := r.q.WithTx(tx)

	if err := qtx.DeleteAllSeriesTagLinks(ctx, seriesID); err != nil {
		return fmt.Errorf("series: clear existing tag links: %w", err)
	}
	now := time.Now().UTC()
	for _, name := range names {
		tagID, err := qtx.UpsertTag(ctx, gen.UpsertTagParams{
			UserID:    userID,
			Name:      name,
			CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("series: upsert tag %q: %w", name, err)
		}
		if err := qtx.InsertSeriesTagLink(ctx, gen.InsertSeriesTagLinkParams{
			SeriesID: seriesID,
			TagID:    tagID,
		}); err != nil {
			return fmt.Errorf("series: insert tag link %q: %w", name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("series: commit tag tx: %w", err)
	}
	return nil
}

// GetSeriesTags returns the sorted tag names attached to the given
// series. Returns an empty slice (never nil) when the series has no
// tags so the JSON wire shape is stable.
func (r *repository) GetSeriesTags(ctx context.Context, seriesID int64) ([]string, error) {
	names, err := r.q.GetSeriesTags(ctx, seriesID)
	if err != nil {
		return nil, fmt.Errorf("series: get tags: %w", err)
	}
	if names == nil {
		names = []string{}
	}
	return names, nil
}

// ListSeriesTagsBatch returns a map of series id to the sorted list of
// tag names, for every series id in the input slice. Series with no
// tags are absent from the result map; the caller initialises the
// per-series Tags field to [] before consulting the map.
func (r *repository) ListSeriesTagsBatch(ctx context.Context, seriesIDs []int64) (map[int64][]string, error) {
	if len(seriesIDs) == 0 {
		return map[int64][]string{}, nil
	}
	rows, err := r.q.ListSeriesTagsBatch(ctx, seriesIDs)
	if err != nil {
		return nil, fmt.Errorf("series: list tags batch: %w", err)
	}
	out := make(map[int64][]string, len(seriesIDs))
	for _, row := range rows {
		out[row.SeriesID] = append(out[row.SeriesID], row.Name)
	}
	return out, nil
}

// listFilterArgs carries the inputs for the hand-rolled tag-filtered
// queries below. The tag-filter queries can't live in sqlc because
// sqlc's slice-expansion path mixes positional `?N` placeholders with
// unnumbered `?` placeholders in a way that breaks under modernc.org's
// SQLite driver. Building the IN-list inline keeps the placeholder
// strategy uniform (all unnumbered `?`s) and avoids the issue.
type listFilterArgs struct {
	userID   int64
	status   string // empty means "no status filter"
	tagNames []string
	limit    int64
	offset   int64
}

// listSummariesFilteredByTags returns the rollup-summary page for the
// user, scoped to the supplied tag set with AND semantics. The status
// filter is optional (zero-length string means "no status filter").
//
// The query mirrors the no-tag variants' SELECT list 1:1 — same
// correlated subqueries for highest_chapter / entry_count /
// rollup_last_captured_at — so the result shape matches what
// summaryFromAllRow / summaryFromStatusRow produce.
func (r *repository) listSummariesFilteredByTags(ctx context.Context, a listFilterArgs) ([]models.SeriesSummary, error) {
	whereStatus, statusArgs := buildStatusClause(a.status)
	placeholders, nameArgs := buildInClause(a.tagNames)

	// #nosec G202 — whereStatus and placeholders are fixed-shape
	// fragments built from buildStatusClause / buildInClause, which
	// only emit literal SQL ("AND s.status = ?") and "?" placeholders.
	// Every user-supplied value still flows through the parameter
	// slice into QueryContext; nothing reaches the SQL text.
	query := `
SELECT
    s.id,
    s.user_id,
    s.title,
    s.status,
    s.rating,
    s.notes,
    s.created_at,
    s.updated_at,
    (SELECT MAX(e.last_chapter) FROM entries e WHERE e.series_id = s.id) AS highest_chapter,
    CAST((SELECT COUNT(*) FROM entries e WHERE e.series_id = s.id) AS INTEGER) AS entry_count,
    (SELECT MAX(e.last_captured_at) FROM entries e WHERE e.series_id = s.id) AS rollup_last_captured_at
FROM series s
WHERE s.user_id = ?` + whereStatus + `
  AND s.id IN (
    SELECT st.series_id FROM series_tag st
    JOIN tag t ON t.id = st.tag_id
    WHERE t.user_id = ?
      AND t.name IN (` + placeholders + `)
    GROUP BY st.series_id
    HAVING COUNT(DISTINCT t.name) = ?
  )
ORDER BY s.updated_at DESC
LIMIT ? OFFSET ?`

	args := make([]interface{}, 0, 4+len(statusArgs)+len(nameArgs))
	args = append(args, a.userID)
	args = append(args, statusArgs...)
	args = append(args, a.userID)
	args = append(args, nameArgs...)
	args = append(args, int64(len(a.tagNames)), a.limit, a.offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("series: list summaries (tags): %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.SeriesSummary, 0)
	for rows.Next() {
		var row gen.ListSeriesAllRow
		if err := rows.Scan(
			&row.ID,
			&row.UserID,
			&row.Title,
			&row.Status,
			&row.Rating,
			&row.Notes,
			&row.CreatedAt,
			&row.UpdatedAt,
			&row.HighestChapter,
			&row.EntryCount,
			&row.RollupLastCapturedAt,
		); err != nil {
			return nil, fmt.Errorf("series: scan summary (tags): %w", err)
		}
		out = append(out, summaryFromAllRow(row))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("series: iterate summaries (tags): %w", err)
	}
	return out, nil
}

// countFilteredByTags returns the total count of series under the
// user that match every supplied tag name (AND semantics) and the
// optional status filter.
func (r *repository) countFilteredByTags(ctx context.Context, a listFilterArgs) (int64, error) {
	whereStatus, statusArgs := buildStatusClause(a.status)
	placeholders, nameArgs := buildInClause(a.tagNames)

	// #nosec G202 — see listSummariesFilteredByTags for the
	// safe-by-construction argument; same helpers are used here.
	query := `
SELECT COUNT(*) FROM series s
WHERE s.user_id = ?` + whereStatus + `
  AND s.id IN (
    SELECT st.series_id FROM series_tag st
    JOIN tag t ON t.id = st.tag_id
    WHERE t.user_id = ?
      AND t.name IN (` + placeholders + `)
    GROUP BY st.series_id
    HAVING COUNT(DISTINCT t.name) = ?
  )`

	args := make([]interface{}, 0, 3+len(statusArgs)+len(nameArgs))
	args = append(args, a.userID)
	args = append(args, statusArgs...)
	args = append(args, a.userID)
	args = append(args, nameArgs...)
	args = append(args, int64(len(a.tagNames)))

	var n int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("series: count (tags): %w", err)
	}
	return n, nil
}

// buildStatusClause returns the additional `AND s.status = ?` fragment
// when status is non-empty, plus the corresponding arg slice. An
// empty status returns ("", nil) — caller drops it into the query
// without parameter list disruption.
func buildStatusClause(status string) (string, []interface{}) {
	if status == "" {
		return "", nil
	}
	return " AND s.status = ?", []interface{}{status}
}

// buildInClause returns a comma-separated placeholder string for the
// supplied names plus the names as a []interface{} ready to splat
// into args. names is guaranteed non-empty by the caller (tag-filter
// dispatch only happens when len(tags) > 0).
func buildInClause(names []string) (string, []interface{}) {
	if len(names) == 0 {
		// Defensive default: an empty IN-list would be invalid SQL, so
		// we substitute NULL which matches nothing — the parent IN
		// predicate then yields zero rows.
		return "NULL", nil
	}
	parts := make([]string, len(names))
	args := make([]interface{}, len(names))
	for i, n := range names {
		parts[i] = "?"
		args[i] = n
	}
	return strings.Join(parts, ", "), args
}

// --- conversion helpers --------------------------------------------------

func seriesFromGen(r gen.Series) models.Series {
	return models.Series{
		ID:        r.ID,
		UserID:    r.UserID,
		Title:     r.Title,
		Status:    r.Status,
		Rating:    nullInt64ToPtr(r.Rating),
		Notes:     r.Notes,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func summaryFromAllRow(r gen.ListSeriesAllRow) models.SeriesSummary {
	return models.SeriesSummary{
		Series: models.Series{
			ID:        r.ID,
			UserID:    r.UserID,
			Title:     r.Title,
			Status:    r.Status,
			Rating:    nullInt64ToPtr(r.Rating),
			Notes:     r.Notes,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		},
		HighestChapter: anyToFloatPtr(r.HighestChapter),
		EntryCount:     r.EntryCount,
		LastCapturedAt: anyToTimePtr(r.RollupLastCapturedAt),
	}
}

func summaryFromStatusRow(r gen.ListSeriesByStatusRow) models.SeriesSummary {
	return models.SeriesSummary{
		Series: models.Series{
			ID:        r.ID,
			UserID:    r.UserID,
			Title:     r.Title,
			Status:    r.Status,
			Rating:    nullInt64ToPtr(r.Rating),
			Notes:     r.Notes,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		},
		HighestChapter: anyToFloatPtr(r.HighestChapter),
		EntryCount:     r.EntryCount,
		LastCapturedAt: anyToTimePtr(r.RollupLastCapturedAt),
	}
}

func summaryFromSummaryRow(r gen.GetSeriesSummaryRow) models.SeriesSummary {
	return models.SeriesSummary{
		Series: models.Series{
			ID:        r.ID,
			UserID:    r.UserID,
			Title:     r.Title,
			Status:    r.Status,
			Rating:    nullInt64ToPtr(r.Rating),
			Notes:     r.Notes,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		},
		HighestChapter: anyToFloatPtr(r.HighestChapter),
		EntryCount:     r.EntryCount,
		LastCapturedAt: anyToTimePtr(r.RollupLastCapturedAt),
	}
}

func nullInt64ToPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func intPtrToNullInt64(p *int) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*p), Valid: true}
}

// anyToFloatPtr handles the sqlc-generated `interface{}` columns from
// correlated subqueries. modernc.org/sqlite returns nil, int64,
// float64, or string depending on affinity; we accept all of them.
func anyToFloatPtr(v interface{}) *float64 {
	switch x := v.(type) {
	case nil:
		return nil
	case float64:
		return &x
	case int64:
		f := float64(x)
		return &f
	case int:
		f := float64(x)
		return &f
	default:
		return nil
	}
}

func anyToTimePtr(v interface{}) *time.Time {
	switch x := v.(type) {
	case nil:
		return nil
	case time.Time:
		if x.IsZero() {
			return nil
		}
		t := x
		return &t
	case string:
		if x == "" {
			return nil
		}
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.999999999Z07:00",
			"2006-01-02 15:04:05",
		} {
			if t, err := time.Parse(layout, x); err == nil {
				return &t
			}
		}
		return nil
	default:
		return nil
	}
}
