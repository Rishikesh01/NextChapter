package series

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
	pg "github.com/enable-it/nextchapter/backend/internal/store/generated/pg"
)

type postgresRepo struct {
	db *sql.DB
	q  *pg.Queries
}

func NewPostgresRepository(db *sql.DB) *postgresRepo {
	return &postgresRepo{db: db, q: pg.New(db)}
}

func (r *postgresRepo) insertSeries(ctx context.Context, p insertSeriesParams) (models.Series, error) {
	row, err := r.q.CreateSeries(ctx, pg.CreateSeriesParams{
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
	return seriesFromPostgres(row), nil
}

func (r *postgresRepo) getSeriesByID(ctx context.Context, userID, seriesID int64) (models.Series, error) {
	row, err := r.q.GetSeriesByID(ctx, pg.GetSeriesByIDParams{ID: seriesID, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Series{}, models.ErrSeriesNotFound
		}
		return models.Series{}, fmt.Errorf("series: get by id: %w", err)
	}
	return seriesFromPostgres(row), nil
}

func (r *postgresRepo) updateSeries(ctx context.Context, p updateSeriesParams) (models.Series, error) {
	row, err := r.q.UpdateSeries(ctx, pg.UpdateSeriesParams{
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
			return models.Series{}, models.ErrSeriesNotFound
		}
		return models.Series{}, fmt.Errorf("series: update: %w", err)
	}
	return seriesFromPostgres(row), nil
}

func (r *postgresRepo) deleteSeries(ctx context.Context, userID, seriesID int64) (int64, error) {
	n, err := r.q.DeleteSeries(ctx, pg.DeleteSeriesParams{ID: seriesID, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("series: delete: %w", err)
	}
	return n, nil
}

func (r *postgresRepo) listSummariesAll(ctx context.Context, p listSummariesAllParams) ([]models.SeriesSummary, error) {
	if len(p.Tags) > 0 {
		return r.listSummariesFilteredByTags(ctx, listFilterArgs{
			userID:   p.UserID,
			tagNames: p.Tags,
			limit:    p.Limit,
			offset:   p.Offset,
		})
	}
	rows, err := r.q.ListSeriesAll(ctx, pg.ListSeriesAllParams{
		UserID: p.UserID,
		Lim:    p.Limit,
		Off:    p.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("series: list all: %w", err)
	}
	out := make([]models.SeriesSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, summaryFromPostgresAllRow(row))
	}
	return out, nil
}

func (r *postgresRepo) listSummariesByStatus(ctx context.Context, p listSummariesByStatusParams) ([]models.SeriesSummary, error) {
	if len(p.Tags) > 0 {
		return r.listSummariesFilteredByTags(ctx, listFilterArgs{
			userID:   p.UserID,
			status:   p.Status,
			tagNames: p.Tags,
			limit:    p.Limit,
			offset:   p.Offset,
		})
	}
	rows, err := r.q.ListSeriesByStatus(ctx, pg.ListSeriesByStatusParams{
		UserID: p.UserID,
		Status: p.Status,
		Lim:    p.Limit,
		Off:    p.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("series: list by status: %w", err)
	}
	out := make([]models.SeriesSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, summaryFromPostgresStatusRow(row))
	}
	return out, nil
}

func (r *postgresRepo) getSummary(ctx context.Context, userID, seriesID int64) (models.SeriesSummary, error) {
	row, err := r.q.GetSeriesSummary(ctx, pg.GetSeriesSummaryParams{ID: seriesID, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.SeriesSummary{}, models.ErrSeriesNotFound
		}
		return models.SeriesSummary{}, fmt.Errorf("series: get summary: %w", err)
	}
	return summaryFromPostgresSummaryRow(row), nil
}

func (r *postgresRepo) countAll(ctx context.Context, p countAllParams) (int64, error) {
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

func (r *postgresRepo) countByStatus(ctx context.Context, p countByStatusParams) (int64, error) {
	if len(p.Tags) > 0 {
		return r.countFilteredByTags(ctx, listFilterArgs{
			userID:   p.UserID,
			status:   p.Status,
			tagNames: p.Tags,
		})
	}
	n, err := r.q.CountSeriesByStatus(ctx, pg.CountSeriesByStatusParams{UserID: p.UserID, Status: p.Status})
	if err != nil {
		return 0, fmt.Errorf("series: count by status: %w", err)
	}
	return n, nil
}

func (r *postgresRepo) setSeriesTags(ctx context.Context, userID, seriesID int64, names []string) error {
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
		tagID, err := qtx.UpsertTag(ctx, pg.UpsertTagParams{
			UserID:    userID,
			Name:      name,
			CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("series: upsert tag %q: %w", name, err)
		}
		if err := qtx.InsertSeriesTagLink(ctx, pg.InsertSeriesTagLinkParams{
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

func (r *postgresRepo) getSeriesTags(ctx context.Context, seriesID int64) ([]string, error) {
	names, err := r.q.GetSeriesTags(ctx, seriesID)
	if err != nil {
		return nil, fmt.Errorf("series: get tags: %w", err)
	}
	if names == nil {
		names = []string{}
	}
	return names, nil
}

func (r *postgresRepo) listSeriesTagsBatch(ctx context.Context, seriesIDs []int64) (map[int64][]string, error) {
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

// listSummariesFilteredByTags is the Postgres-flavoured tag-filter query.
// Postgres requires `$N` indexed placeholders rather than SQLite's
// unnumbered `?` form, and the COUNT(*) result inside the IN-subquery
// needs an explicit cast to compare against a bigint placeholder.
func (r *postgresRepo) listSummariesFilteredByTags(ctx context.Context, a listFilterArgs) ([]models.SeriesSummary, error) {
	pb := newPgPlaceholders()
	userIDIdx := pb.next()

	statusFrag := ""
	if a.status != "" {
		statusFrag = " AND s.status = " + pb.next()
	}

	userIDIdx2 := pb.next()

	tagPlaceholders := make([]string, 0, len(a.tagNames))
	for range a.tagNames {
		tagPlaceholders = append(tagPlaceholders, pb.next())
	}

	tagCountIdx := pb.next()
	limitIdx := pb.next()
	offsetIdx := pb.next()

	// #nosec G202 — every placeholder string is generated locally by
	// the pgPlaceholders builder; the only user-supplied values flow
	// through args into QueryContext. Nothing here is interpolated
	// from caller input.
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
    (SELECT COUNT(*) FROM entries e WHERE e.series_id = s.id)::bigint AS entry_count,
    (SELECT MAX(e.last_captured_at) FROM entries e WHERE e.series_id = s.id) AS rollup_last_captured_at
FROM series s
WHERE s.user_id = ` + userIDIdx + statusFrag + `
  AND s.id IN (
    SELECT st.series_id FROM series_tag st
    JOIN tag t ON t.id = st.tag_id
    WHERE t.user_id = ` + userIDIdx2 + `
      AND t.name IN (` + strings.Join(tagPlaceholders, ", ") + `)
    GROUP BY st.series_id
    HAVING COUNT(DISTINCT t.name) = ` + tagCountIdx + `
  )
ORDER BY s.updated_at DESC
LIMIT ` + limitIdx + ` OFFSET ` + offsetIdx

	args := make([]interface{}, 0, 4+len(a.tagNames))
	args = append(args, a.userID)
	if a.status != "" {
		args = append(args, a.status)
	}
	args = append(args, a.userID)
	for _, n := range a.tagNames {
		args = append(args, n)
	}
	args = append(args, int64(len(a.tagNames)), a.limit, a.offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("series: list summaries (tags): %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.SeriesSummary, 0)
	for rows.Next() {
		var row pg.ListSeriesAllRow
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
		out = append(out, summaryFromPostgresAllRow(row))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("series: iterate summaries (tags): %w", err)
	}
	return out, nil
}

func (r *postgresRepo) countFilteredByTags(ctx context.Context, a listFilterArgs) (int64, error) {
	pb := newPgPlaceholders()
	userIDIdx := pb.next()

	statusFrag := ""
	if a.status != "" {
		statusFrag = " AND s.status = " + pb.next()
	}

	userIDIdx2 := pb.next()

	tagPlaceholders := make([]string, 0, len(a.tagNames))
	for range a.tagNames {
		tagPlaceholders = append(tagPlaceholders, pb.next())
	}

	tagCountIdx := pb.next()

	// #nosec G202 — same safe-by-construction argument as the list variant.
	query := `
SELECT COUNT(*) FROM series s
WHERE s.user_id = ` + userIDIdx + statusFrag + `
  AND s.id IN (
    SELECT st.series_id FROM series_tag st
    JOIN tag t ON t.id = st.tag_id
    WHERE t.user_id = ` + userIDIdx2 + `
      AND t.name IN (` + strings.Join(tagPlaceholders, ", ") + `)
    GROUP BY st.series_id
    HAVING COUNT(DISTINCT t.name) = ` + tagCountIdx + `
  )`

	args := make([]interface{}, 0, 3+len(a.tagNames))
	args = append(args, a.userID)
	if a.status != "" {
		args = append(args, a.status)
	}
	args = append(args, a.userID)
	for _, n := range a.tagNames {
		args = append(args, n)
	}
	args = append(args, int64(len(a.tagNames)))

	var n int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("series: count (tags): %w", err)
	}
	return n, nil
}

// pgPlaceholders generates the $1, $2, ... sequence Postgres expects.
// Bare ints are easier to reason about than tracking the next-index by
// hand at every concatenation site.
type pgPlaceholders struct {
	n int
}

func newPgPlaceholders() *pgPlaceholders { return &pgPlaceholders{} }

func (p *pgPlaceholders) next() string {
	p.n++
	return "$" + strconv.Itoa(p.n)
}

// --- Postgres row -> domain model converters ------------------------------

func seriesFromPostgres(r pg.Series) models.Series {
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

func summaryFromPostgresAllRow(r pg.ListSeriesAllRow) models.SeriesSummary {
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

func summaryFromPostgresStatusRow(r pg.ListSeriesByStatusRow) models.SeriesSummary {
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

func summaryFromPostgresSummaryRow(r pg.GetSeriesSummaryRow) models.SeriesSummary {
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
