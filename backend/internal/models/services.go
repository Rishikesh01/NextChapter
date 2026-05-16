package models

import "context"

// AuthService is the surface the HTTP handlers consume for auth-token
// lifecycle (session mint/revoke on login/logout, API token mint/revoke
// for the extension). Resolve / Touch are middleware-internal and stay
// off this interface.
type AuthService interface {
	CreateSession(ctx context.Context, userID int64) (SessionToken, error)
	DeleteSession(ctx context.Context, rawToken string) error
	CreateAPI(ctx context.Context, userID int64, p NewToken) (APIToken, error)
	DeleteAPI(ctx context.Context, userID, tokenID int64) (bool, error)
}

// UsersService is the surface the HTTP handlers consume for user
// account lifecycle. Authenticate returns the public-facing [User]
// shape — PasswordHash never crosses this boundary.
type UsersService interface {
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, p Registration) (User, error)
	Authenticate(ctx context.Context, p Credentials) (User, error)
}

// SeriesService is the surface the HTTP handlers consume for the
// series CRUD endpoints.
type SeriesService interface {
	Create(ctx context.Context, userID int64, p SeriesNew) (Series, error)
	List(ctx context.Context, userID int64, p SeriesFilter) ([]SeriesSummary, int64, error)
	Get(ctx context.Context, userID, id int64) (Series, error)
	Detail(ctx context.Context, userID, id int64) (SeriesDetail, error)
	Update(ctx context.Context, userID, id int64, p SeriesPatch) (Series, error)
	Delete(ctx context.Context, userID, id int64) error
}

// EntriesService is the surface the HTTP handlers consume for the
// entries endpoints. Capture returns (entry, created, err) — the bool
// distinguishes the 201 vs 200 paths on the wire.
type EntriesService interface {
	Capture(ctx context.Context, userID int64, p EntryCapture, sc SeriesCreator) (Entry, bool, error)
	List(ctx context.Context, userID int64, p EntryFilter) ([]Entry, int64, error)
	Patch(ctx context.Context, userID, entryID int64, p EntryPatch) (Entry, error)
	Delete(ctx context.Context, userID, entryID int64) error
}
