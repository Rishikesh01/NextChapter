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
