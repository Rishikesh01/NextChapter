package models

import "time"

// SeriesCoverMeta is the metadata half of a stored cover: everything
// except the image bytes. It is the wire shape for PUT
// /series/{id}/cover and the return of the series service's
// SetSeriesCover method. The bytes never appear on a JSON response —
// they are served only by GET /series/{id}/cover as an image body.
type SeriesCoverMeta struct {
	SeriesID  int64     `json:"series_id"`
	UserID    int64     `json:"-"`
	Mime      string    `json:"mime"`
	ByteSize  int64     `json:"byte_size"`
	Width     int64     `json:"width"`
	Height    int64     `json:"height"`
	ETag      string    `json:"etag"`
	SourceURL string    `json:"source_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SeriesCover is a stored cover including its image bytes. Internal
// only: the handler writes Bytes as the response body with the Mime
// content type and never serialises this struct. Every field is
// `json:"-"` so an accidental c.JSON emits `{}` rather than
// base64-blasting an image into a JSON payload.
type SeriesCover struct {
	Meta  SeriesCoverMeta `json:"-"`
	Bytes []byte          `json:"-"`
}

// CoverUpload is the input to the series service's SetSeriesCover
// method: the raw uploaded bytes plus the optional page URL they came
// from. The service — not the caller — decides whether the bytes are
// an acceptable image; the declared Content-Type of the request is
// deliberately not part of this struct, because the backend sniffs the
// type from the bytes themselves (ADR-0011 §4).
type CoverUpload struct {
	Bytes []byte
	// SourceURL is the page or image URL the extension pulled the
	// bytes from. Stored for provenance only — the backend never
	// fetches it, and nothing in the product dereferences it.
	SourceURL string
}
