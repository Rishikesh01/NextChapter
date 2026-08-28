package models

import "errors"

// Domain error sentinels surfaced across the handler / service
// boundary. Handlers compare against these via errors.Is.
//
// Services are expected to return the canonical sentinel directly
// (not a wrapped private one) when the failure mode matches one of
// these meanings. Wrapping is fine as long as errors.Is unwraps.
var (
	// ErrUsernameTaken is returned by the users service's Register
	// method when the username collides with an existing row.
	// Handlers turn this into 422.
	ErrUsernameTaken = errors.New("users: username already taken")

	// ErrUserNotFound is returned by the auth service's Authenticate
	// method when no row exists for the supplied username. Handlers
	// collapse this with ErrInvalidCredentials into a single 401 to
	// prevent account enumeration.
	ErrUserNotFound = errors.New("users: not found")

	// ErrInvalidCredentials is returned by the auth service's
	// Authenticate method on a bcrypt mismatch. See ErrUserNotFound
	// for the 401 collapse.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")

	// ErrSeriesNotFound is returned by SeriesService.FindSeries /
	// EditSeries / UntrackSeries / InspectSeries and by
	// EntriesService.CaptureChapter / AdjustReadingPosition when a
	// series id is unknown or belongs to another user. Handlers turn
	// this into 404 (for series endpoints) or 422 (for entries
	// endpoints, where the bad id is in the body).
	ErrSeriesNotFound = errors.New("series: not found")

	// ErrSeriesInvalidStatus is returned by SeriesService when a
	// status value is not in the openapi enum. Handlers turn this
	// into 422.
	ErrSeriesInvalidStatus = errors.New("series: invalid status")

	// ErrEntryNotFound is returned by
	// EntriesService.AdjustReadingPosition /
	// EntriesService.ForgetReadingPosition when the entry id is
	// unknown or belongs to another user. Handlers turn this into 404.
	ErrEntryNotFound = errors.New("entries: not found")

	// ErrEntryCaptureSeriesRequired is returned by
	// EntriesService.CaptureChapter when no entry exists for the (host,
	// slug) key and the caller supplied neither SeriesID nor
	// NewSeriesTitle. Handlers turn this into 422.
	ErrEntryCaptureSeriesRequired = errors.New("entries: series_id or new_series_title required when creating")

	// ErrSiteRuleNotFound is returned by SitesService.EditSiteRule /
	// RemoveSiteRule when the rule id is unknown or belongs to another
	// user. Handlers turn this into 404.
	ErrSiteRuleNotFound = errors.New("sites: rule not found")

	// ErrSiteRuleInvalidRegex is returned by SitesService.AddSiteRule /
	// EditSiteRule when the supplied ChapterURLRegex does not compile
	// as a Go regexp. Handlers turn this into 422 with a field-level
	// error on chapter_url_regex.
	ErrSiteRuleInvalidRegex = errors.New("sites: chapter_url_regex must compile as a Go regexp")

	// ErrSiteRuleMissingCaptureGroup is returned by
	// SitesService.AddSiteRule / EditSiteRule when one of the
	// configured capture-group names is not present as a named
	// sub-expression in the compiled regex. Handlers turn this into
	// 422 with a field-level error on slug_capture_group and/or
	// chapter_capture_group.
	ErrSiteRuleMissingCaptureGroup = errors.New("sites: capture group is missing from chapter_url_regex")

	// ErrSiteRuleHostTaken is returned by SitesService.AddSiteRule when
	// a rule for the same (user, host) pair already exists. Handlers
	// turn this into 422 with a field-level error on host.
	ErrSiteRuleHostTaken = errors.New("sites: a rule for this host already exists")

	// ErrCoverNotFound is returned by SeriesService.FindSeriesCover /
	// RemoveSeriesCover when the series has no cover stored. Handlers
	// turn this into 404.
	ErrCoverNotFound = errors.New("series: cover not found")

	// ErrCoverUnsupportedType is returned by
	// SeriesService.SetSeriesCover when the uploaded bytes are not
	// JPEG, PNG or WebP. The type is sniffed from the bytes, never
	// taken from the request's Content-Type (ADR-0011 §4). Handlers
	// turn this into 422.
	ErrCoverUnsupportedType = errors.New("series: cover must be a JPEG, PNG or WebP image")

	// ErrCoverUndecodable is returned by SeriesService.SetSeriesCover
	// when the bytes sniff as a supported image type but the decoder
	// cannot read their dimensions — a truncated or corrupt upload.
	// Handlers turn this into 422.
	ErrCoverUndecodable = errors.New("series: cover image could not be decoded")

	// ErrCoverEmpty is returned by SeriesService.SetSeriesCover when
	// the request body carried no bytes. Handlers turn this into 422.
	ErrCoverEmpty = errors.New("series: cover image is empty")
)
