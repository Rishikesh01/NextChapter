package models

import (
	"context"
	"time"
)

// Entry mirrors the API "Entry" schema. It is the value type passed
// across the handler / service / persistence boundaries.
type Entry struct {
	ID             int64
	UserID         int64
	SeriesID       int64
	SiteHost       string
	SeriesSlug     string
	SiteTitle      string
	LastChapter    float64
	LastURL        string
	LastCapturedAt time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// EntryCapture is both the POST /entries/capture JSON body and the
// input to the entries service's Capture method. Chapter is a
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
// to the entries service's Patch method. Pointer fields use the
// standard absent/present binary: nil means "leave the column alone".
type EntryPatch struct {
	SeriesID    *int64   `json:"series_id,omitempty"    binding:"omitempty,min=1"`
	LastChapter *float64 `json:"last_chapter,omitempty" binding:"omitempty,min=0"`
	LastURL     *string  `json:"last_url,omitempty"`
	SiteTitle   *string  `json:"site_title,omitempty"   binding:"omitempty,min=1,max=512"`
}

// EntryFilter paginates the entries list.
type EntryFilter struct {
	SeriesID *int64
	Limit    int
	Offset   int
}

// SeriesCreator is the narrow seam the entries service uses when
// capture is called with new_series_title rather than series_id. The
// concrete implementation is wired in at router-build time by the
// series service. It lives here because both the entries service's
// Capture method and the entries package depend on it, and we want
// the wire shape to stay in [models].
type SeriesCreator interface {
	Create(ctx context.Context, userID int64, title string) (int64, error)
}
