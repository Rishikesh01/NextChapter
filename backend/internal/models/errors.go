package models

import "errors"

// Domain error sentinels surfaced across the handler / service
// boundary. Handlers compare against these via errors.Is so they do
// not have to import the domain packages.
//
// Services are expected to return the canonical sentinel directly
// (not a wrapped private one) when the failure mode matches one of
// these meanings. Wrapping is fine as long as errors.Is unwraps.
var (
	// ErrUsernameTaken is returned by [UsersService.Create] when the
	// username collides with an existing row. Handlers turn this into 422.
	ErrUsernameTaken = errors.New("users: username already taken")

	// ErrUserNotFound is returned by [UsersService.Authenticate] when
	// no row exists for the supplied username. Handlers collapse this
	// with ErrInvalidCredentials into a single 401 to prevent account
	// enumeration.
	ErrUserNotFound = errors.New("users: not found")

	// ErrInvalidCredentials is returned by [UsersService.Authenticate]
	// on a bcrypt mismatch. See ErrUserNotFound for the 401 collapse.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")

	// ErrSeriesNotFound is returned by SeriesService.Get / Update /
	// Delete / Detail and by EntriesService.Capture / Patch when a
	// series id is unknown or belongs to another user. Handlers turn
	// this into 404 (for series endpoints) or 422 (for entries
	// endpoints, where the bad id is in the body).
	ErrSeriesNotFound = errors.New("series: not found")

	// ErrSeriesInvalidStatus is returned by SeriesService when a
	// status value is not in the openapi enum. Handlers turn this
	// into 422.
	ErrSeriesInvalidStatus = errors.New("series: invalid status")

	// ErrEntryNotFound is returned by EntriesService.Patch / Delete
	// when the entry id is unknown or belongs to another user.
	// Handlers turn this into 404.
	ErrEntryNotFound = errors.New("entries: not found")

	// ErrEntryCaptureSeriesRequired is returned by
	// EntriesService.Capture when no entry exists for the (host,
	// slug) key and the caller supplied neither SeriesID nor
	// NewSeriesTitle. Handlers turn this into 422.
	ErrEntryCaptureSeriesRequired = errors.New("entries: series_id or new_series_title required when creating")
)
