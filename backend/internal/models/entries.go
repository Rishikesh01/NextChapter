package models

import (
	"context"
	"time"
)

// Entry mirrors the API "Entry" schema and is the value type passed
// across the handler / service / persistence boundaries. Handlers
// return Entry directly via c.JSON; UserID is internal scoping
// metadata and is tagged `json:"-"` so it never reaches the wire.
type Entry struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"-"`
	SeriesID       int64     `json:"series_id"`
	SiteHost       string    `json:"site_host"`
	SeriesSlug     string    `json:"series_slug"`
	SiteTitle      string    `json:"site_title"`
	LastChapter    float64   `json:"last_chapter"`
	LastURL        string    `json:"last_url"`
	LastCapturedAt time.Time `json:"last_captured_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// EntryList is the wire envelope for GET /entries: a page of entries
// plus the total count for the filtered set.
type EntryList struct {
	Items []Entry `json:"items"`
	Total int64   `json:"total"`
}

// EntryCapture is both the POST /entries/capture JSON body and the
// input to the entries service's CaptureChapter method. Chapter is a
// *float64 so that a missing field is distinguishable from chapter 0
// (which is a valid value); the `required` binding tag rejects the
// missing case.
type EntryCapture struct {
	SiteHost       string   `json:"site_host"       binding:"required,min=1,max=253"`
	SeriesSlug     string   `json:"series_slug"     binding:"required,min=1,max=512"`
	SiteTitle      string   `json:"site_title"      binding:"required,min=1,max=512"`
	Chapter        *float64 `json:"chapter"         binding:"required,gte=0"`
	URL            string   `json:"url"             binding:"required,min=1,max=2048,url"`
	SeriesID       *int64   `json:"series_id"       binding:"omitempty,min=1"`
	NewSeriesTitle *string  `json:"new_series_title"`
}

// EntryPatch is both the PATCH /entries/{id} JSON body and the input
// to the entries service's AdjustReadingPosition method. Pointer
// fields use the standard absent/present binary: nil means "leave the
// column alone".
type EntryPatch struct {
	SeriesID    *int64   `json:"series_id,omitempty"    binding:"omitempty,min=1"`
	LastChapter *float64 `json:"last_chapter,omitempty" binding:"omitempty,min=0"`
	LastURL     *string  `json:"last_url,omitempty"`
	SiteTitle   *string  `json:"site_title,omitempty"   binding:"omitempty,min=1,max=512"`
}

// EntryFilter binds the GET /entries query string and feeds the
// entries service's list method. Pagination defaults and bounds live
// here, in the binding layer, not in the service — out-of-range
// values surface as 422 with a field-level error via the handler's
// standard validator.ValidationErrors path.
//
// The literal 50 / 200 / 0 duplicate [constants.ListLimitDefault] /
// [constants.ListLimitMax] / [constants.ListOffsetMin]; Go struct
// tags can't reference constants. Update both when the bounds change.
type EntryFilter struct {
	SeriesID *int64 `form:"series_id"        binding:"omitempty,min=1"`
	Limit    int    `form:"limit,default=50" binding:"omitempty,min=1,max=200"`
	Offset   int    `form:"offset,default=0" binding:"omitempty,min=0"`
}

// SeriesCreator is the narrow seam the entries service uses when
// capture is called with new_series_title rather than series_id. The
// concrete implementation is wired in at router-build time by the
// series service. It lives here because both the entries service's
// CaptureChapter method and the entries package depend on it, and we
// want the wire shape to stay in [models].
type SeriesCreator interface {
	Create(ctx context.Context, userID int64, title string) (int64, error)
}
