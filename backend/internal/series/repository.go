package series

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// NewRepository builds the concrete Repository backed by a *gen.Queries.
func NewRepository(q *gen.Queries) Repository {
	return &repository{q: q}
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

func (r *repository) CountAll(ctx context.Context, userID int64) (int64, error) {
	n, err := r.q.CountSeriesAll(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("series: count all: %w", err)
	}
	return n, nil
}

func (r *repository) CountByStatus(ctx context.Context, userID int64, status string) (int64, error) {
	n, err := r.q.CountSeriesByStatus(ctx, gen.CountSeriesByStatusParams{UserID: userID, Status: status})
	if err != nil {
		return 0, fmt.Errorf("series: count by status: %w", err)
	}
	return n, nil
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
		ID:             r.ID,
		UserID:         r.UserID,
		Title:          r.Title,
		Status:         r.Status,
		Rating:         nullInt64ToPtr(r.Rating),
		Notes:          r.Notes,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		HighestChapter: anyToFloatPtr(r.HighestChapter),
		EntryCount:     r.EntryCount,
		LastCapturedAt: anyToTimePtr(r.RollupLastCapturedAt),
	}
}

func summaryFromStatusRow(r gen.ListSeriesByStatusRow) models.SeriesSummary {
	return models.SeriesSummary{
		ID:             r.ID,
		UserID:         r.UserID,
		Title:          r.Title,
		Status:         r.Status,
		Rating:         nullInt64ToPtr(r.Rating),
		Notes:          r.Notes,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		HighestChapter: anyToFloatPtr(r.HighestChapter),
		EntryCount:     r.EntryCount,
		LastCapturedAt: anyToTimePtr(r.RollupLastCapturedAt),
	}
}

func summaryFromSummaryRow(r gen.GetSeriesSummaryRow) models.SeriesSummary {
	return models.SeriesSummary{
		ID:             r.ID,
		UserID:         r.UserID,
		Title:          r.Title,
		Status:         r.Status,
		Rating:         nullInt64ToPtr(r.Rating),
		Notes:          r.Notes,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
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
