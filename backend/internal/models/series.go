package models

import "time"

// Series is the domain shape of a series row, returned by Create /
// Update / Get. The summary queries return [SeriesSummary] which
// carries the rollup columns on top.
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

// SeriesSummary is the listing shape returned by GET /series.
type SeriesSummary struct {
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

// SeriesDetail is the shape returned by GET /series/{id}: the summary
// plus the full per-site entry list.
type SeriesDetail struct {
	SeriesSummary
	Entries []Entry
}

// SeriesNew is both the POST /series JSON body and the input to
// [SeriesService.Create]. Field bounds duplicate the numbers in
// [constants.SeriesTitleMin] / [constants.SeriesTitleMax] /
// [constants.RatingMin] / [constants.RatingMax] /
// [constants.SeriesNotesMax] because Go struct tags can't reference
// constants. Update both when the bounds change.
type SeriesNew struct {
	Title  string `json:"title"            binding:"required,min=1,max=256"`
	Status string `json:"status,omitempty" binding:"omitempty,oneof=reading completed on_hold dropped plan_to_read"`
	Rating *int   `json:"rating,omitempty" binding:"omitempty,min=1,max=10"`
	Notes  string `json:"notes,omitempty"  binding:"max=8192"`
}

// SeriesPatch is both the PATCH /series/{id} JSON body and the input
// to [SeriesService.Update]. Each pointer field is the standard
// absent/present binary: nil means "leave the column alone". The v1
// API does not support *clearing* the rating column via PATCH —
// `rating: null` on the wire is treated the same as the field being
// absent. If clearing becomes a product requirement it will be a
// separate endpoint, not a side-effect of PATCH. Bounds mirror
// [SeriesNew].
type SeriesPatch struct {
	Title  *string `json:"title,omitempty"  binding:"omitempty,min=1,max=256"`
	Status *string `json:"status,omitempty" binding:"omitempty,oneof=reading completed on_hold dropped plan_to_read"`
	Rating *int    `json:"rating,omitempty" binding:"omitempty,min=1,max=10"`
	Notes  *string `json:"notes,omitempty"  binding:"omitempty,max=8192"`
}

// SeriesFilter configures a paginated list of summaries.
type SeriesFilter struct {
	Status string // optional; "" = all statuses
	Limit  int
	Offset int
}
