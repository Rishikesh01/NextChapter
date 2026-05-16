// Package constants holds shared enum values, string identifiers, and
// numeric bounds that cross package boundaries inside the backend. The
// rule of thumb: if the same literal would otherwise appear in two
// places (service code, handler validation, or a test), it belongs here.
//
// Files in this package are organised by topic. Nothing in this package
// has runtime dependencies; it imports only the standard library where
// strictly necessary.
package constants

// Series status enum. Mirrors the CHECK constraint on series.status and
// the SeriesStatus enum in the OpenAPI spec; keep these three sources
// in sync when the enum changes.
const (
	StatusReading    = "reading"
	StatusCompleted  = "completed"
	StatusOnHold     = "on_hold"
	StatusDropped    = "dropped"
	StatusPlanToRead = "plan_to_read"
)

// DefaultSeriesStatus is applied when a SeriesCreateRequest omits the
// status field. Kept here so any code that needs to "fill in the
// default" reads from a single named value.
const DefaultSeriesStatus = StatusReading

// AllSeriesStatuses lists every valid status value in the order they
// should appear in user-facing prompts/messages. Callers that want a
// set/lookup map should build one once at init time.
var AllSeriesStatuses = []string{
	StatusReading,
	StatusCompleted,
	StatusOnHold,
	StatusDropped,
	StatusPlanToRead,
}

// Field bounds for series.* columns. Numbers mirror the OpenAPI schema
// (`Series` definition); update both in the same change.
const (
	// SeriesTitleMin is the inclusive lower bound on series.title length
	// after trimming.
	SeriesTitleMin = 1
	// SeriesTitleMax is the inclusive upper bound on series.title length.
	SeriesTitleMax = 256

	// SeriesNotesMax is the inclusive upper bound on series.notes length.
	// notes has no minimum — the empty string is the unset value.
	SeriesNotesMax = 8192

	// RatingMin / RatingMax are the inclusive bounds on series.rating,
	// the 1-10 manga-style rating column.
	RatingMin = 1
	RatingMax = 10
)
