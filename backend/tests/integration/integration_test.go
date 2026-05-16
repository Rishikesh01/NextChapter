package integration_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/auth"
	"github.com/enable-it/nextchapter/backend/internal/entries"
	"github.com/enable-it/nextchapter/backend/internal/httpapi/handlers"
	"github.com/enable-it/nextchapter/backend/internal/series"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
	"github.com/enable-it/nextchapter/backend/internal/users"
)

// intPtr is a tiny helper for expected-body DTO literals where a *int
// field needs a concrete value (e.g. Rating: intPtr(8)).
func intPtr(v int) *int { return &v }

// errorBody is the canonical error envelope produced by the render
// package. Used as ExpectedBody when the handler returns a structured
// error response.
type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

var (
	notFoundBody     = errorBody{Error: errorPayload{Code: "not_found", Message: "not found"}}
	unauthorisedBody = errorBody{Error: errorPayload{Code: "unauthorized", Message: "missing or invalid credentials"}}
)

// TestSelfHostedOperatorBootsAndRegistersFirstUser walks the
// open-registration window: a fresh DB lets exactly one register call
// succeed and then closes the window forever. The created user is
// queryable via /auth/me using the cookie set by /auth/register, and
// the row lands in the users table.
func TestSelfHostedOperatorBootsAndRegistersFirstUser(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newHarness(t)

	(testRequest{
		Name:           "first /auth/register on fresh DB returns 201",
		Method:         http.MethodPost,
		Path:           "/auth/register",
		Body:           users.RegisterParams{Username: "alice", Password: "correct horse battery"},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   handlers.UserResponse{Username: "alice"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)

	row, err := h.queries.GetUserByUsername(context.Background(), "alice")
	r.NoError(err)
	r.Equal("alice", row.Username)

	(testRequest{
		Name:           "second /auth/register call closes the window with 404",
		Method:         http.MethodPost,
		Path:           "/auth/register",
		Body:           users.RegisterParams{Username: "mallory", Password: "another password"},
		ExpectedStatus: http.StatusNotFound,
		ExpectedBody:   notFoundBody,
	}).do(t, h)

	n, err := h.queries.CountUsers(context.Background())
	r.NoError(err)
	r.Equal(int64(1), n)

	(testRequest{
		Name:           "/auth/me echoes the registered user via the session cookie",
		Method:         http.MethodGet,
		Path:           "/auth/me",
		ExpectedStatus: http.StatusOK,
		ExpectedBody:   handlers.UserResponse{Username: "alice"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)

	// Capture the raw session cookie before logout so we can assert the
	// persisted row is gone afterwards.
	preCookies := h.client.Jar.Cookies(parseURL(t, h.srv.URL))
	var preLogout string
	for _, ck := range preCookies {
		if ck.Name == constants.SessionCookieName {
			preLogout = ck.Value
			break
		}
	}
	r.NotEmpty(preLogout, "expected a session cookie before /auth/logout")
	preHash := auth.HashToken(preLogout)
	_, err = h.queries.GetAuthTokenByHash(context.Background(), preHash)
	r.NoError(err, "session row should exist before logout")

	(testRequest{
		Name:           "logout clears the session",
		Method:         http.MethodPost,
		Path:           "/auth/logout",
		ExpectedStatus: http.StatusNoContent,
	}).do(t, h)

	// Store-state: the auth_tokens row for this hash must be gone.
	_, err = h.queries.GetAuthTokenByHash(context.Background(), preHash)
	r.True(errors.Is(err, sql.ErrNoRows),
		"session row should be deleted by /auth/logout (got err=%v)", err)

	(testRequest{
		Name:           "/auth/me after logout is 401",
		Method:         http.MethodGet,
		Path:           "/auth/me",
		ExpectedStatus: http.StatusUnauthorized,
		ExpectedBody:   unauthorisedBody,
	}).do(t, h)
}

// TestEnvVarBootstrapClosesRegistrationFromTheStart is the second
// bootstrap path: when NEXTCHAPTER_BOOTSTRAP_USERNAME/_PASSWORD are
// set, the user is created on first boot and /auth/register is 404
// without ever having served a single registration.
func TestEnvVarBootstrapClosesRegistrationFromTheStart(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	cfg := defaultCfg()
	cfg.BootstrapUsername = "alice"
	cfg.BootstrapPassword = "correct horse battery"
	h := startServer(t, cfg)

	(testRequest{
		Name:           "/auth/register is closed",
		Method:         http.MethodPost,
		Path:           "/auth/register",
		Body:           users.RegisterParams{Username: "mallory", Password: "another password"},
		ExpectedStatus: http.StatusNotFound,
		ExpectedBody:   notFoundBody,
	}).do(t, h)

	(testRequest{
		Name:           "/auth/login with the bootstrap creds succeeds",
		Method:         http.MethodPost,
		Path:           "/auth/login",
		Body:           auth.LoginParams{Username: "alice", Password: "correct horse battery"},
		ExpectedStatus: http.StatusOK,
		ExpectedBody:   handlers.UserResponse{Username: "alice"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)

	row, err := h.queries.GetUserByUsername(context.Background(), "alice")
	r.NoError(err)

	// Store-state: login must have produced a kind=session row for
	// the bootstrap user.
	sessions, err := h.queries.ListSessionTokens(context.Background(), row.ID)
	r.NoError(err)
	r.Len(sessions, 1, "expected exactly one session token after /auth/login")
	r.Equal(constants.TokenKindSession, sessions[0].Kind)
	r.Equal(row.ID, sessions[0].UserID)

	(testRequest{
		Name:           "/auth/me confirms the session is live",
		Method:         http.MethodGet,
		Path:           "/auth/me",
		ExpectedStatus: http.StatusOK,
		ExpectedBody:   handlers.UserResponse{Username: "alice"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)
}

// TestUserCapturesChapterProgressAcrossSites covers the canonical
// product flow: log in, create a series, capture a chapter, advance
// it, attempt to rewind (no-op), capture from a different site, then
// check the series rollups.
func TestUserCapturesChapterProgressAcrossSites(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	aliceUID := aliceID(t, h)

	// Pre-condition: one series under the logged-in user. Single
	// request, no Name — runs inline.
	_, seriesBody := (testRequest{
		Method:         http.MethodPost,
		Path:           "/series",
		Body:           series.CreateParams{Title: "Solo Leveling", Status: constants.StatusReading},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   handlers.SeriesResponse{Title: "Solo Leveling", Status: constants.StatusReading},
		SentinelPaths:  []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	seriesID := decodeSeriesID(t, seriesBody)

	row := requireSeriesRow(t, h, seriesID)
	r.Equal("Solo Leveling", row.Title)
	r.Equal(constants.StatusReading, row.Status)
	r.False(row.Rating.Valid)

	chapter100 := 100.0
	chapter110 := 110.5
	chapter90 := 90.0
	chapter105 := 105.0

	_, body := (testRequest{
		Name:   "first capture creates an entry",
		Method: http.MethodPost,
		Path:   "/entries/capture",
		Body: entries.CaptureParams{
			SiteHost:   "reader.example.com",
			SeriesSlug: "solo-leveling",
			SiteTitle:  "Solo Leveling",
			Chapter:    &chapter100,
			URL:        "https://reader.example.com/series/solo-leveling/chapter-100",
			SeriesID:   &seriesID,
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: handlers.EntryResponse{
			SeriesID:    seriesID,
			SiteHost:    "reader.example.com",
			SeriesSlug:  "solo-leveling",
			SiteTitle:   "Solo Leveling",
			LastChapter: 100,
			LastURL:     "https://reader.example.com/series/solo-leveling/chapter-100",
		},
		SentinelPaths: []string{"id", "last_captured_at", "created_at", "updated_at"},
	}).do(t, h)
	firstEntryID := decodeEntryID(t, body)

	entryRow, err := h.queries.GetEntryByID(context.Background(), gen.GetEntryByIDParams{ID: firstEntryID, UserID: aliceUID})
	r.NoError(err)
	r.Equal(100.0, entryRow.LastChapter)
	r.Equal("reader.example.com", entryRow.SiteHost)
	r.Equal(seriesID, entryRow.SeriesID)

	(testRequest{
		Name:   "re-capture at higher chapter advances last_chapter",
		Method: http.MethodPost,
		Path:   "/entries/capture",
		Body: entries.CaptureParams{
			SiteHost:   "reader.example.com",
			SeriesSlug: "solo-leveling",
			SiteTitle:  "Solo Leveling",
			Chapter:    &chapter110,
			URL:        "https://reader.example.com/series/solo-leveling/chapter-110.5",
		},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: handlers.EntryResponse{
			ID:          firstEntryID,
			SeriesID:    seriesID,
			SiteHost:    "reader.example.com",
			SeriesSlug:  "solo-leveling",
			SiteTitle:   "Solo Leveling",
			LastChapter: 110.5,
			LastURL:     "https://reader.example.com/series/solo-leveling/chapter-110.5",
		},
		SentinelPaths: []string{"last_captured_at", "created_at", "updated_at"},
	}).do(t, h)

	entryRow, err = h.queries.GetEntryByID(context.Background(), gen.GetEntryByIDParams{ID: firstEntryID, UserID: aliceUID})
	r.NoError(err)
	r.Equal(110.5, entryRow.LastChapter)

	// Rewind is a no-op on last_chapter / last_url / site_title:
	// the response should mirror the pre-rewind row (last_chapter
	// still 110.5, last_url still the chapter-110.5 url) but
	// last_captured_at and updated_at do bump (sentinelled out).
	(testRequest{
		Name:   "rewind capture is a no-op on last_chapter",
		Method: http.MethodPost,
		Path:   "/entries/capture",
		Body: entries.CaptureParams{
			SiteHost:   "reader.example.com",
			SeriesSlug: "solo-leveling",
			SiteTitle:  "Solo Leveling",
			Chapter:    &chapter90,
			URL:        "https://reader.example.com/series/solo-leveling/chapter-90",
		},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: handlers.EntryResponse{
			ID:          firstEntryID,
			SeriesID:    seriesID,
			SiteHost:    "reader.example.com",
			SeriesSlug:  "solo-leveling",
			SiteTitle:   "Solo Leveling",
			LastChapter: 110.5,
			LastURL:     "https://reader.example.com/series/solo-leveling/chapter-110.5",
		},
		SentinelPaths: []string{"last_captured_at", "created_at", "updated_at"},
	}).do(t, h)

	entryRow, err = h.queries.GetEntryByID(context.Background(), gen.GetEntryByIDParams{ID: firstEntryID, UserID: aliceUID})
	r.NoError(err)
	r.Equal(110.5, entryRow.LastChapter)

	_, body = (testRequest{
		Name:   "second-site capture allocates a new entry row",
		Method: http.MethodPost,
		Path:   "/entries/capture",
		Body: entries.CaptureParams{
			SiteHost:   "comics.example.org",
			SeriesSlug: "solo-leveling-i-alone-level-up",
			SiteTitle:  "Solo Leveling (I Alone Level Up)",
			Chapter:    &chapter105,
			URL:        "https://comics.example.org/comic/solo-leveling-i-alone-level-up/chapter-105",
			SeriesID:   &seriesID,
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: handlers.EntryResponse{
			SeriesID:    seriesID,
			SiteHost:    "comics.example.org",
			SeriesSlug:  "solo-leveling-i-alone-level-up",
			SiteTitle:   "Solo Leveling (I Alone Level Up)",
			LastChapter: 105,
			LastURL:     "https://comics.example.org/comic/solo-leveling-i-alone-level-up/chapter-105",
		},
		SentinelPaths: []string{"id", "last_captured_at", "created_at", "updated_at"},
	}).do(t, h)
	secondEntryID := decodeEntryID(t, body)
	r.NotEqual(firstEntryID, secondEntryID, "second-site capture must allocate a new entry row")

	total, err := h.queries.CountEntriesBySeries(context.Background(), gen.CountEntriesBySeriesParams{
		UserID: aliceUID, SeriesID: seriesID,
	})
	r.NoError(err)
	r.Equal(int64(2), total)

	// last_captured_at on the rollup is currently null: the
	// underlying correlated MAX() column comes back as []byte from
	// modernc.org/sqlite which the conversion shim does not parse.
	// That's the actual wire shape today, and the test pins it
	// verbatim — if the shim is fixed we want this to fail loudly.
	highest := 110.5
	(testRequest{
		Name:           "/series list rolls up highest_chapter and entry_count",
		Method:         http.MethodGet,
		Path:           "/series",
		ExpectedStatus: http.StatusOK,
		ExpectedBody: handlers.SeriesListResponse{
			Items: []handlers.SeriesSummaryResponse{{
				SeriesResponse: handlers.SeriesResponse{
					Title:  "Solo Leveling",
					Status: constants.StatusReading,
				},
				HighestChapter: &highest,
				EntryCount:     2,
				LastCapturedAt: nil,
			}},
			Total: 1,
		},
		SentinelPaths: []string{
			"items.*.id",
			"items.*.created_at",
			"items.*.updated_at",
		},
	}).do(t, h)
}

// TestUserReassignsEntryBetweenSeries covers the manual-correction
// flow: when the operator captured under the wrong series (or wants
// to split a long-running series at a "season" boundary), they PATCH
// /entries/{id} with a new series_id. The data layer must reflect
// the move on both ends.
func TestUserReassignsEntryBetweenSeries(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	aliceUID := aliceID(t, h)

	// Setup: two series, one entry attached to s1.
	// The Create handler defaults status to constants.DefaultSeriesStatus when none is
	// supplied; the expected body must reflect that wire shape.
	_, s1Body := (testRequest{
		Name:           "seed series 1",
		Method:         http.MethodPost,
		Path:           "/series",
		Body:           series.CreateParams{Title: "Solo Leveling"},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   handlers.SeriesResponse{Title: "Solo Leveling", Status: constants.StatusReading},
		SentinelPaths:  []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	s1ID := decodeSeriesID(t, s1Body)

	_, s2Body := (testRequest{
		Name:           "seed series 2",
		Method:         http.MethodPost,
		Path:           "/series",
		Body:           series.CreateParams{Title: "Solo Leveling (continuation)"},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   handlers.SeriesResponse{Title: "Solo Leveling (continuation)", Status: constants.StatusReading},
		SentinelPaths:  []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	s2ID := decodeSeriesID(t, s2Body)

	chapter100 := 100.0
	_, eBody := (testRequest{
		Name:   "seed entry on series 1",
		Method: http.MethodPost,
		Path:   "/entries/capture",
		Body: entries.CaptureParams{
			SiteHost:   "reader.example.com",
			SeriesSlug: "solo-leveling",
			SiteTitle:  "Solo Leveling",
			Chapter:    &chapter100,
			URL:        "https://reader.example.com/series/solo-leveling/chapter-100",
			SeriesID:   &s1ID,
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: handlers.EntryResponse{
			SeriesID:    s1ID,
			SiteHost:    "reader.example.com",
			SeriesSlug:  "solo-leveling",
			SiteTitle:   "Solo Leveling",
			LastChapter: 100,
			LastURL:     "https://reader.example.com/series/solo-leveling/chapter-100",
		},
		SentinelPaths: []string{"id", "last_captured_at", "created_at", "updated_at"},
	}).do(t, h)
	entryID := decodeEntryID(t, eBody)

	(testRequest{
		Name:           "PATCH /entries/{id} returns the updated entry with the new series_id",
		Method:         http.MethodPatch,
		Path:           fmt.Sprintf("/entries/%d", entryID),
		Body:           entries.UpdateParams{SeriesID: &s2ID},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: handlers.EntryResponse{
			ID:          entryID,
			SeriesID:    s2ID,
			SiteHost:    "reader.example.com",
			SeriesSlug:  "solo-leveling",
			SiteTitle:   "Solo Leveling",
			LastChapter: 100,
			LastURL:     "https://reader.example.com/series/solo-leveling/chapter-100",
		},
		SentinelPaths: []string{"last_captured_at", "created_at", "updated_at"},
	}).do(t, h)

	// Store-state: the entry row carries the new series_id and the
	// rollup counts on both ends move accordingly.
	entryRow, err := h.queries.GetEntryByID(context.Background(), gen.GetEntryByIDParams{ID: entryID, UserID: aliceUID})
	r.NoError(err)
	r.Equal(s2ID, entryRow.SeriesID)

	oldTotal, err := h.queries.CountEntriesBySeries(context.Background(), gen.CountEntriesBySeriesParams{
		UserID: aliceUID, SeriesID: s1ID,
	})
	r.NoError(err)
	r.Equal(int64(0), oldTotal, "old series rollup should lose the entry")

	newTotal, err := h.queries.CountEntriesBySeries(context.Background(), gen.CountEntriesBySeriesParams{
		UserID: aliceUID, SeriesID: s2ID,
	})
	r.NoError(err)
	r.Equal(int64(1), newTotal, "new series rollup should gain the entry")

	highest := 100.0
	// Same rollup quirk as in TestUserCapturesChapterProgressAcrossSites:
	// last_captured_at at the summary level is nil today.
	(testRequest{
		Name:           "GET /series/{new} includes the reassigned entry",
		Method:         http.MethodGet,
		Path:           fmt.Sprintf("/series/%d", s2ID),
		ExpectedStatus: http.StatusOK,
		ExpectedBody: handlers.SeriesDetailResponse{
			SeriesSummaryResponse: handlers.SeriesSummaryResponse{
				SeriesResponse: handlers.SeriesResponse{
					ID:     s2ID,
					Title:  "Solo Leveling (continuation)",
					Status: constants.StatusReading,
				},
				HighestChapter: &highest,
				EntryCount:     1,
				LastCapturedAt: nil,
			},
			Entries: []handlers.EntryResponse{{
				ID:          entryID,
				SeriesID:    s2ID,
				SiteHost:    "reader.example.com",
				SeriesSlug:  "solo-leveling",
				SiteTitle:   "Solo Leveling",
				LastChapter: 100,
				LastURL:     "https://reader.example.com/series/solo-leveling/chapter-100",
			}},
		},
		SentinelPaths: []string{
			"created_at",
			"updated_at",
			"entries.*.last_captured_at",
			"entries.*.created_at",
			"entries.*.updated_at",
		},
	}).do(t, h)
}

// TestAPITokenBearerFlow covers the extension flow: cookie-mint a
// token, use it via Bearer on a bare client, revoke it, verify
// subsequent Bearer calls 401. There is no list endpoint on
// /auth/tokens; mint returns the raw token exactly once and the
// server only stores its hash thereafter.
func TestAPITokenBearerFlow(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	aliceUID := aliceID(t, h)
	bareClient := h.bareClient()

	_, body := (testRequest{
		Name:           "minting a token via cookie session returns raw token once",
		Method:         http.MethodPost,
		Path:           "/auth/tokens",
		Body:           auth.CreateTokenParams{Label: "extension"},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: handlers.APITokenCreatedResponse{
			APITokenResponse: handlers.APITokenResponse{
				Label:      "extension",
				LastUsedAt: nil,
				ExpiresAt:  nil,
			},
			// Token is non-deterministic; the sentinel path puts
			// "<token>" on both sides.
			Token: "<token>",
		},
		SentinelPaths: []string{"id", "created_at", "token"},
	}).do(t, h)
	tokenID, rawToken := decodeMintedToken(t, body)

	r.True(strings.HasPrefix(rawToken, constants.TokenPrefixAPI),
		"token must carry %q prefix: %q", constants.TokenPrefixAPI, rawToken)

	// Store-state: a kind=api row exists for alice with the right
	// label, and its token_hash equals sha256(plaintext) re-derived
	// from the response body. The plaintext is never persisted in
	// the test fixture. This lookup goes through h.queries directly
	// (the only API surface for listing API tokens is removed; the
	// underlying sqlc query stays so the test can still assert).
	expectedHash := hex.EncodeToString(func() []byte {
		s := sha256.Sum256([]byte(rawToken))
		return s[:]
	}())
	rows, err := h.queries.ListAPITokens(context.Background(), aliceUID)
	r.NoError(err)
	r.Len(rows, 1, "expected one API token row after mint")
	r.Equal(constants.TokenKindAPI, rows[0].Kind)
	r.Equal("extension", rows[0].Label.String)
	r.True(rows[0].Label.Valid)
	r.Equal(expectedHash, rows[0].TokenHash, "token_hash must equal sha256(plaintext)")

	(testRequest{
		Name:           "bare Bearer client can hit /auth/me",
		Method:         http.MethodGet,
		Path:           "/auth/me",
		Headers:        http.Header{"Authorization": []string{"Bearer " + rawToken}},
		Client:         bareClient,
		ExpectedStatus: http.StatusOK,
		ExpectedBody:   handlers.UserResponse{Username: "alice"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)

	(testRequest{
		Name:           "revoking the token returns 204",
		Method:         http.MethodDelete,
		Path:           fmt.Sprintf("/auth/tokens/%d", tokenID),
		ExpectedStatus: http.StatusNoContent,
	}).do(t, h)

	rows, err = h.queries.ListAPITokens(context.Background(), aliceUID)
	r.NoError(err)
	r.Empty(rows)

	(testRequest{
		Name:           "subsequent Bearer call returns 401",
		Method:         http.MethodGet,
		Path:           "/auth/me",
		Headers:        http.Header{"Authorization": []string{"Bearer " + rawToken}},
		Client:         bareClient,
		ExpectedStatus: http.StatusUnauthorized,
		ExpectedBody:   unauthorisedBody,
	}).do(t, h)
}

// TestHealthzIsOpenAndReturnsExpectedBody pins the meta endpoint:
// unauthenticated, 200, body shape exact.
func TestHealthzIsOpenAndReturnsExpectedBody(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	(testRequest{
		Method:         http.MethodGet,
		Path:           "/healthz",
		ExpectedStatus: http.StatusOK,
		ExpectedBody:   handlers.HealthResponse{Status: "ok", Version: "test"},
	}).do(t, h)
}

// TestProtectedRouteRequiresAuthentication is one of the new
// coverage-gap tests: hitting a protected endpoint without any
// credentials should 401 with the unified envelope.
func TestProtectedRouteRequiresAuthentication(t *testing.T) {
	t.Parallel()
	h := startServer(t, bootstrappedCfg())
	bare := h.bareClient()

	for _, path := range []string{"/auth/me", "/series", "/entries"} {
		(testRequest{
			Name:           "GET " + path + " without credentials is 401",
			Method:         http.MethodGet,
			Path:           path,
			Client:         bare,
			ExpectedStatus: http.StatusUnauthorized,
			ExpectedBody:   unauthorisedBody,
		}).do(t, h)
	}
}

// TestTokenKindSeparationIsEnforced verifies that a session-prefixed
// token (ncs_...) supplied via Authorization: Bearer is not silently
// accepted. The middleware must reject prefix/kind mismatches.
func TestTokenKindSeparationIsEnforced(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)

	// Reach into the cookie jar to recover the raw session token Alice
	// has from logging in.
	cookies := h.client.Jar.Cookies(parseURL(t, h.srv.URL))
	var sessionToken string
	for _, ck := range cookies {
		if ck.Name == constants.SessionCookieName {
			sessionToken = ck.Value
			break
		}
	}
	r.NotEmpty(sessionToken, "expected %s cookie on the authenticated client", constants.SessionCookieName)
	r.True(strings.HasPrefix(sessionToken, constants.TokenPrefixSession),
		"session cookie should carry %q prefix", constants.TokenPrefixSession)

	(testRequest{
		Method:         http.MethodGet,
		Path:           "/auth/me",
		Headers:        http.Header{"Authorization": []string{"Bearer " + sessionToken}},
		Client:         h.bareClient(),
		ExpectedStatus: http.StatusUnauthorized,
		ExpectedBody:   unauthorisedBody,
	}).do(t, h)
}

// TestPatchSeriesFieldSemantics pins the absent vs value semantics on
// PATCH /series/{id}: an absent field is left alone, a present field
// updates the column, and `rating: null` on the wire is a no-op
// (matches "absent"). The v1 API does not allow clearing rating via
// PATCH — see [series.UpdateParams].
func TestPatchSeriesFieldSemantics(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	aliceUID := aliceID(t, h)

	// Seed: rating=8, notes="initial", status=reading.
	_, seedBody := (testRequest{
		Name:   "seed series with rating + notes",
		Method: http.MethodPost,
		Path:   "/series",
		Body: series.CreateParams{
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusReading,
			Rating: intPtr(8),
			Notes:  "initial",
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: handlers.SeriesResponse{
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusReading,
			Rating: intPtr(8),
			Notes:  "initial",
		},
		SentinelPaths: []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	seriesID := decodeSeriesID(t, seedBody)

	patchPath := fmt.Sprintf("/series/%d", seriesID)

	notes := "now with thoughts"
	(testRequest{
		Name:           "updating notes leaves rating and status untouched",
		Method:         http.MethodPatch,
		Path:           patchPath,
		Body:           series.UpdateParams{Notes: &notes},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: handlers.SeriesResponse{
			ID:     seriesID,
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusReading,
			Rating: intPtr(8),
			Notes:  "now with thoughts",
		},
		SentinelPaths: []string{"created_at", "updated_at"},
	}).do(t, h)

	row := requireSeriesRow(t, h, seriesID)
	r.Equal("now with thoughts", row.Notes)
	r.True(row.Rating.Valid)
	r.Equal(int64(8), row.Rating.Int64)

	// `rating: null` on the wire is a no-op under the v1 API: it
	// deserialises to a nil *int, which the handler treats the same as
	// the field being absent. The existing rating must be preserved.
	// Build the body via map[string]any so we send literal `null` on
	// the wire (typed encoding with omitempty would otherwise drop the
	// key entirely).
	(testRequest{
		Name:           "rating: null is a no-op; existing rating is preserved",
		Method:         http.MethodPatch,
		Path:           patchPath,
		Body:           map[string]any{"rating": nil},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: handlers.SeriesResponse{
			ID:     seriesID,
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusReading,
			Rating: intPtr(8),
			Notes:  "now with thoughts",
		},
		SentinelPaths: []string{"created_at", "updated_at"},
	}).do(t, h)

	preservedRow, err := h.queries.GetSeriesByID(context.Background(), gen.GetSeriesByIDParams{ID: seriesID, UserID: aliceUID})
	r.NoError(err)
	r.True(preservedRow.Rating.Valid, "rating: null must not clear the column under the v1 API")
	r.Equal(int64(8), preservedRow.Rating.Int64)

	status := constants.StatusCompleted
	(testRequest{
		Name:           "status update flips the column",
		Method:         http.MethodPatch,
		Path:           patchPath,
		Body:           series.UpdateParams{Status: &status},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: handlers.SeriesResponse{
			ID:     seriesID,
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusCompleted,
			Rating: intPtr(8),
			Notes:  "now with thoughts",
		},
		SentinelPaths: []string{"created_at", "updated_at"},
	}).do(t, h)

	finalRow := requireSeriesRow(t, h, seriesID)
	r.Equal(constants.StatusCompleted, finalRow.Status)
}

// TestCaptureNormalisesSiteHost pins ADR-0005's host-normalisation
// contract on POST /entries/capture: the inbound SiteHost is
// lowercased and the leading "www." (case-insensitive) is stripped
// before the (user_id, site_host, series_slug) upsert key is computed.
// Two captures against the same logical site must therefore hit the
// same entry row regardless of whether the client sends
// "WWW.reader.example.com" or the canonical "reader.example.com".
func TestCaptureNormalisesSiteHost(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	aliceUID := aliceID(t, h)

	_, seriesBody := (testRequest{
		Name:           "seed series for host-normalisation capture",
		Method:         http.MethodPost,
		Path:           "/series",
		Body:           series.CreateParams{Title: "Solo Leveling (host-norm)", Status: constants.StatusReading},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   handlers.SeriesResponse{Title: "Solo Leveling (host-norm)", Status: constants.StatusReading},
		SentinelPaths:  []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	seriesID := decodeSeriesID(t, seriesBody)

	c1 := 1.0
	url1 := fmt.Sprintf("https://reader.example.com/series/solo-leveling-host-norm/chapter-%d", 1)
	_, body := (testRequest{
		Name:   "first capture normalises SiteHost to lowercase no-www",
		Method: http.MethodPost,
		Path:   "/entries/capture",
		Body: entries.CaptureParams{
			SiteHost:   "WWW.reader.example.com",
			SeriesSlug: "solo-leveling-host-norm",
			SiteTitle:  "Solo Leveling",
			Chapter:    &c1,
			URL:        url1,
			SeriesID:   &seriesID,
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: handlers.EntryResponse{
			SeriesID:    seriesID,
			SiteHost:    "reader.example.com",
			SeriesSlug:  "solo-leveling-host-norm",
			SiteTitle:   "Solo Leveling",
			LastChapter: 1,
			LastURL:     url1,
		},
		SentinelPaths: []string{"id", "last_captured_at", "created_at", "updated_at"},
	}).do(t, h)
	entryID := decodeEntryID(t, body)

	// Load-bearing: the persisted column must be normalised, not just
	// the response payload.
	row, err := h.queries.GetEntryByID(context.Background(), gen.GetEntryByIDParams{ID: entryID, UserID: aliceUID})
	r.NoError(err)
	r.Equal("reader.example.com", row.SiteHost)

	c2 := 5.0
	url2 := fmt.Sprintf("https://reader.example.com/series/solo-leveling-host-norm/chapter-%d", 5)
	(testRequest{
		Name:   "second capture with canonical host hits the same entry",
		Method: http.MethodPost,
		Path:   "/entries/capture",
		Body: entries.CaptureParams{
			SiteHost:   "reader.example.com",
			SeriesSlug: "solo-leveling-host-norm",
			SiteTitle:  "Solo Leveling",
			Chapter:    &c2,
			URL:        url2,
		},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: handlers.EntryResponse{
			ID:          entryID,
			SeriesID:    seriesID,
			SiteHost:    "reader.example.com",
			SeriesSlug:  "solo-leveling-host-norm",
			SiteTitle:   "Solo Leveling",
			LastChapter: 5,
			LastURL:     url2,
		},
		SentinelPaths: []string{"last_captured_at", "created_at", "updated_at"},
	}).do(t, h)

	total, err := h.queries.CountEntriesBySeries(context.Background(), gen.CountEntriesBySeriesParams{
		UserID: aliceUID, SeriesID: seriesID,
	})
	r.NoError(err)
	r.Equal(int64(1), total, "both captures must share a single entry row")
}
