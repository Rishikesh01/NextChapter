package entries

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// NewRepository builds the concrete Repository backed by a *gen.Queries.
func NewRepository(q *gen.Queries) Repository {
	return &repository{q: q}
}

func (r *repository) GetEntryByID(ctx context.Context, userID, id int64) (Entry, error) {
	row, err := r.q.GetEntryByID(ctx, gen.GetEntryByIDParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, ErrNotFound
		}
		return Entry{}, fmt.Errorf("entries: get by id: %w", err)
	}
	return entryFromGen(row), nil
}

func (r *repository) GetEntryByKey(ctx context.Context, p GetEntryByKeyParams) (Entry, error) {
	row, err := r.q.GetEntryByKey(ctx, gen.GetEntryByKeyParams{
		UserID:     p.UserID,
		SiteHost:   p.SiteHost,
		SeriesSlug: p.SeriesSlug,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, ErrNotFound
		}
		return Entry{}, fmt.Errorf("entries: get by key: %w", err)
	}
	return entryFromGen(row), nil
}

func (r *repository) InsertEntry(ctx context.Context, p InsertEntryParams) (Entry, error) {
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
		return Entry{}, fmt.Errorf("entries: insert: %w", err)
	}
	return entryFromGen(row), nil
}

func (r *repository) AdvanceEntry(ctx context.Context, p AdvanceEntryParams) (Entry, error) {
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
		return Entry{}, fmt.Errorf("entries: advance: %w", err)
	}
	return entryFromGen(row), nil
}

func (r *repository) UpdateEntry(ctx context.Context, p UpdateEntryParams) (Entry, error) {
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
			return Entry{}, ErrNotFound
		}
		return Entry{}, fmt.Errorf("entries: update: %w", err)
	}
	return entryFromGen(row), nil
}

func (r *repository) DeleteEntry(ctx context.Context, userID, id int64) (int64, error) {
	n, err := r.q.DeleteEntry(ctx, gen.DeleteEntryParams{ID: id, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("entries: delete: %w", err)
	}
	return n, nil
}

func (r *repository) ListEntriesAll(ctx context.Context, p ListEntriesAllParams) ([]Entry, error) {
	rows, err := r.q.ListEntriesAll(ctx, gen.ListEntriesAllParams{
		UserID: p.UserID,
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("entries: list all: %w", err)
	}
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entryFromGen(row))
	}
	return out, nil
}

func (r *repository) ListEntriesBySeries(ctx context.Context, p ListEntriesBySeriesParams) ([]Entry, error) {
	rows, err := r.q.ListEntriesBySeries(ctx, gen.ListEntriesBySeriesParams{
		UserID:   p.UserID,
		SeriesID: p.SeriesID,
		Limit:    p.Limit,
		Offset:   p.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("entries: list by series: %w", err)
	}
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entryFromGen(row))
	}
	return out, nil
}

func (r *repository) ListEntriesAllForSeries(ctx context.Context, userID, seriesID int64) ([]Entry, error) {
	rows, err := r.q.ListEntriesAllForSeries(ctx, gen.ListEntriesAllForSeriesParams{
		UserID:   userID,
		SeriesID: seriesID,
	})
	if err != nil {
		return nil, fmt.Errorf("entries: list all for series: %w", err)
	}
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entryFromGen(row))
	}
	return out, nil
}

func (r *repository) CountEntriesAll(ctx context.Context, userID int64) (int64, error) {
	n, err := r.q.CountEntriesAll(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("entries: count all: %w", err)
	}
	return n, nil
}

func (r *repository) CountEntriesBySeries(ctx context.Context, userID, seriesID int64) (int64, error) {
	n, err := r.q.CountEntriesBySeries(ctx, gen.CountEntriesBySeriesParams{
		UserID:   userID,
		SeriesID: seriesID,
	})
	if err != nil {
		return 0, fmt.Errorf("entries: count by series: %w", err)
	}
	return n, nil
}

func (r *repository) SeriesExists(ctx context.Context, userID, seriesID int64) (bool, error) {
	v, err := r.q.SeriesExists(ctx, gen.SeriesExistsParams{ID: seriesID, UserID: userID})
	if err != nil {
		return false, fmt.Errorf("entries: series exists: %w", err)
	}
	return v != 0, nil
}

func entryFromGen(r gen.Entry) Entry {
	return Entry{
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
