package entries

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

type sqliteRepo struct {
	q *gen.Queries
}

func newSQLiteRepository(db *sql.DB) *sqliteRepo {
	return &sqliteRepo{q: gen.New(db)}
}

func (r *sqliteRepo) GetEntryByID(ctx context.Context, userID, entryID int64) (models.Entry, error) {
	row, err := r.q.GetEntryByID(ctx, gen.GetEntryByIDParams{ID: entryID, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Entry{}, ErrNotFound
		}
		return models.Entry{}, fmt.Errorf("entries: get by id: %w", err)
	}
	return entryFromSQLite(row), nil
}

func (r *sqliteRepo) GetEntryByKey(ctx context.Context, p GetEntryByKeyParams) (models.Entry, error) {
	row, err := r.q.GetEntryByKey(ctx, gen.GetEntryByKeyParams{
		UserID:     p.UserID,
		SiteHost:   p.SiteHost,
		SeriesSlug: p.SeriesSlug,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Entry{}, ErrNotFound
		}
		return models.Entry{}, fmt.Errorf("entries: get by key: %w", err)
	}
	return entryFromSQLite(row), nil
}

func (r *sqliteRepo) InsertEntry(ctx context.Context, p InsertEntryParams) (models.Entry, error) {
	row, err := r.q.CreateEntry(ctx, gen.CreateEntryParams{
		UserID:         p.UserID,
		SeriesID:       p.SeriesID,
		SiteHost:       p.SiteHost,
		SeriesSlug:     p.SeriesSlug,
		SiteTitle:      p.SiteTitle,
		LastChapter:    p.LastChapter,
		LastUrl:        p.LastURL,
		LastCapturedAt: p.LastCapturedAt,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	})
	if err != nil {
		return models.Entry{}, fmt.Errorf("entries: insert: %w", err)
	}
	return entryFromSQLite(row), nil
}

func (r *sqliteRepo) AdvanceEntry(ctx context.Context, p AdvanceEntryParams) (models.Entry, error) {
	row, err := r.q.AdvanceEntry(ctx, gen.AdvanceEntryParams{
		LastChapter:    p.LastChapter,
		LastUrl:        p.LastURL,
		SiteTitle:      p.SiteTitle,
		LastCapturedAt: p.LastCapturedAt,
		UpdatedAt:      p.UpdatedAt,
		ID:             p.ID,
		UserID:         p.UserID,
	})
	if err != nil {
		return models.Entry{}, fmt.Errorf("entries: advance: %w", err)
	}
	return entryFromSQLite(row), nil
}

func (r *sqliteRepo) UpdateEntry(ctx context.Context, p UpdateEntryParams) (models.Entry, error) {
	row, err := r.q.UpdateEntry(ctx, gen.UpdateEntryParams{
		SeriesID:    p.SeriesID,
		LastChapter: p.LastChapter,
		LastUrl:     p.LastURL,
		SiteTitle:   p.SiteTitle,
		UpdatedAt:   p.UpdatedAt,
		ID:          p.ID,
		UserID:      p.UserID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Entry{}, ErrNotFound
		}
		return models.Entry{}, fmt.Errorf("entries: update: %w", err)
	}
	return entryFromSQLite(row), nil
}

func (r *sqliteRepo) DeleteEntry(ctx context.Context, userID, entryID int64) (int64, error) {
	n, err := r.q.DeleteEntry(ctx, gen.DeleteEntryParams{ID: entryID, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("entries: delete: %w", err)
	}
	return n, nil
}

func (r *sqliteRepo) ListEntriesAll(ctx context.Context, p ListEntriesAllParams) ([]models.Entry, error) {
	rows, err := r.q.ListEntriesAll(ctx, gen.ListEntriesAllParams{
		UserID: p.UserID,
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("entries: list all: %w", err)
	}
	out := make([]models.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entryFromSQLite(row))
	}
	return out, nil
}

func (r *sqliteRepo) ListEntriesBySeries(ctx context.Context, p ListEntriesBySeriesParams) ([]models.Entry, error) {
	rows, err := r.q.ListEntriesBySeries(ctx, gen.ListEntriesBySeriesParams{
		UserID:   p.UserID,
		SeriesID: p.SeriesID,
		Limit:    p.Limit,
		Offset:   p.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("entries: list by series: %w", err)
	}
	out := make([]models.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entryFromSQLite(row))
	}
	return out, nil
}

func (r *sqliteRepo) ListEntriesAllForSeries(ctx context.Context, userID, seriesID int64) ([]models.Entry, error) {
	rows, err := r.q.ListEntriesAllForSeries(ctx, gen.ListEntriesAllForSeriesParams{
		UserID:   userID,
		SeriesID: seriesID,
	})
	if err != nil {
		return nil, fmt.Errorf("entries: list all for series: %w", err)
	}
	out := make([]models.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entryFromSQLite(row))
	}
	return out, nil
}

func (r *sqliteRepo) CountEntriesAll(ctx context.Context, userID int64) (int64, error) {
	n, err := r.q.CountEntriesAll(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("entries: count all: %w", err)
	}
	return n, nil
}

func (r *sqliteRepo) CountEntriesBySeries(ctx context.Context, userID, seriesID int64) (int64, error) {
	n, err := r.q.CountEntriesBySeries(ctx, gen.CountEntriesBySeriesParams{
		UserID:   userID,
		SeriesID: seriesID,
	})
	if err != nil {
		return 0, fmt.Errorf("entries: count by series: %w", err)
	}
	return n, nil
}

func (r *sqliteRepo) SeriesExists(ctx context.Context, userID, seriesID int64) (bool, error) {
	v, err := r.q.SeriesExists(ctx, gen.SeriesExistsParams{ID: seriesID, UserID: userID})
	if err != nil {
		return false, fmt.Errorf("entries: series exists: %w", err)
	}
	return v != 0, nil
}

func (r *sqliteRepo) ListTrackedHosts(ctx context.Context, userID int64) ([]string, error) {
	hosts, err := r.q.ListTrackedHosts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("entries: list tracked hosts: %w", err)
	}
	if hosts == nil {
		hosts = []string{}
	}
	return hosts, nil
}

func entryFromSQLite(r gen.Entry) models.Entry {
	return models.Entry{
		ID:             r.ID,
		UserID:         r.UserID,
		SeriesID:       r.SeriesID,
		SiteHost:       r.SiteHost,
		SeriesSlug:     r.SeriesSlug,
		SiteTitle:      r.SiteTitle,
		LastChapter:    r.LastChapter,
		LastURL:        r.LastUrl,
		LastCapturedAt: r.LastCapturedAt,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}
