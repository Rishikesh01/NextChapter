package models

import "time"

// Series is the domain shape of a series row and the wire shape for
// POST/PATCH /series. The Create / Update / Get paths return this
// directly; the summary listing returns [SeriesSummary] which embeds
// Series and carries the rollup columns on top. UserID is internal
// scoping metadata — never on the wire.
//
// Tags is a sorted list of user-defined free-text labels attached to
// the series. The repository / service layer guarantees the slice is
// non-nil and lexicographically sorted before it reaches the wire, so
// the JSON payload is `[]` (never `null`) and test fixtures are
// deterministic.
type Series struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"-"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Rating    *int      `json:"rating"`
	Notes     string    `json:"notes"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SeriesSummary is the per-row wire shape returned by GET /series.
// Embedding [Series] flattens the base columns to top-level fields on
// the JSON payload; the three rollup fields appear alongside them.
type SeriesSummary struct {
	Series
	HighestChapter *float64   `json:"highest_chapter"`
	EntryCount     int64      `json:"entry_count"`
	LastCapturedAt *time.Time `json:"last_captured_at"`
	// CoverUpdatedAt is nil when the series has no cover image. It is
	// both the existence flag (so the grid renders a placeholder
	// without issuing a doomed request per coverless series) and the
	// cache-buster the client appends to the cover URL (ADR-0011 §6).
	CoverUpdatedAt *time.Time `json:"cover_updated_at"`
}

// SeriesDetail is the wire shape for GET /series/{id}: the summary
// plus the full per-site entry list. Entries serialise inside the
// detail response with the same shape as the standalone /entries
// endpoint because [Entry] carries the same json tags.
type SeriesDetail struct {
	SeriesSummary
	Entries []Entry `json:"entries"`
}

// SeriesList is the wire envelope for GET /series: a page of
// summaries plus the total count for the filtered set.
type SeriesList struct {
	Items []SeriesSummary `json:"items"`
	Total int64           `json:"total"`
}

// SeriesNew is both the POST /series JSON body and the input to the
// series service's Create method.
//
// Tags is an optional list of user-defined free-text labels. The
// `tagname` validator enforces the per-tag pattern
// (^[a-z0-9][a-z0-9-]{0,31}$); max=16 caps the per-series count.
type SeriesNew struct {
	Title  string   `json:"title"            binding:"required,min=1,max=256"`
	Status string   `json:"status,omitempty" binding:"omitempty,oneof=reading completed on_hold dropped plan_to_read"`
	Rating *int     `json:"rating,omitempty" binding:"omitempty,min=1,max=10"`
	Notes  string   `json:"notes,omitempty"  binding:"max=8192"`
	Tags   []string `json:"tags,omitempty"   binding:"omitempty,max=16,dive,tagname"`
}

// SeriesPatch is both the PATCH /series/{id} JSON body and the input
// to the series service's Update method. Each pointer field is the
// standard absent/present binary: nil means "leave the column alone".
// The v1 API does not support *clearing* the rating column via PATCH —
// `rating: null` on the wire is treated the same as the field being
// absent. If clearing becomes a product requirement it will be a
// separate endpoint, not a side-effect of PATCH. Bounds mirror
// [SeriesNew].
//
// Tags is a `*[]string` so the three states are distinguishable:
//   - field absent (nil pointer): existing tags left alone.
//   - empty list (`{"tags": []}`): remove every tag from the series.
//   - non-empty list: replace the current tag set with the supplied list.
//
// This is canonical full-replace semantics — there is no per-tag
// add/remove on this endpoint.
type SeriesPatch struct {
	Title  *string   `json:"title,omitempty"  binding:"omitempty,min=1,max=256"`
	Status *string   `json:"status,omitempty" binding:"omitempty,oneof=reading completed on_hold dropped plan_to_read"`
	Rating *int      `json:"rating,omitempty" binding:"omitempty,min=1,max=10"`
	Notes  *string   `json:"notes,omitempty"  binding:"omitempty,max=8192"`
	Tags   *[]string `json:"tags,omitempty"   binding:"omitempty,max=16,dive,tagname"`
}

// SeriesFilter binds the GET /series query string and feeds the
// series service's list method. Pagination defaults and bounds live
// here, in the binding layer, not in the service — out-of-range
// values surface as 422 with a field-level error via the handler's
// standard validator.ValidationErrors path.
//
// Tags applies an AND-semantic filter — a series only appears in the
// result if it carries every supplied tag. The `?tag=foo&tag=bar`
// repeated-query-string form binds into the slice via gin's
// form-binding rules.
type SeriesFilter struct {
	Status string   `form:"status"           binding:"omitempty,oneof=reading completed on_hold dropped plan_to_read"`
	Limit  int      `form:"limit,default=50" binding:"omitempty,min=1,max=200"`
	Offset int      `form:"offset,default=0" binding:"omitempty,min=0"`
	Tags   []string `form:"tag"              binding:"omitempty,max=16,dive,tagname"`
}
