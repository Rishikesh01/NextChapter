package entries

import "errors"

// ErrNotFound is returned when a lookup misses, or the row belongs to
// another user. Handlers turn this into 404.
var ErrNotFound = errors.New("entries: not found")

// ErrSeriesRequired is returned by Capture when no entry exists for the
// (host, slug) key and the caller supplied neither SeriesID nor
// NewSeriesTitle. Handlers turn this into 422.
var ErrSeriesRequired = errors.New("entries: series_id or new_series_title required when creating")

// ErrSeriesNotFound is returned when Capture or Update is asked to attach
// to a series_id that doesn't exist or belongs to another user.
var ErrSeriesNotFound = errors.New("entries: series not found")
