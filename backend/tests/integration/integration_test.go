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
	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
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
	unauthorisedBody = errorBody{Error: errorPayload{Code: "unauthorized", Message: "missing or invalid credentials"}}
)

// TestUserRegisterAndLogin walks the open-registration flow: a fresh
// DB accepts the register call, the resulting session cookie powers
// /auth/me, logout clears it, and /auth/me afterwards is 401. /auth/register
// remains open so a second, distinct account also registers cleanly.
func TestUserRegisterAndLogin(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newHarness(t)

	(testRequest{
		Name:           "first /auth/register on fresh DB returns 201",
		Method:         http.MethodPost,
		Path:           "/auth/register",
		Body:           models.Registration{Username: "alice", Password: "correct horse battery"},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   models.User{Username: "alice"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)

	row, err := h.queries.GetUserByUsername(context.Background(), "alice")
	r.NoError(err)
	r.Equal("alice", row.Username)

	(testRequest{
		Name:           "/auth/me echoes the registered user via the session cookie",
		Method:         http.MethodGet,
		Path:           "/auth/me",
		ExpectedStatus: http.StatusOK,
		ExpectedBody:   models.User{Username: "alice"},
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

	(testRequest{
		Name:           "a second distinct user can also register",
		Method:         http.MethodPost,
		Path:           "/auth/register",
		Body:           models.Registration{Username: "bob", Password: "another password"},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   models.User{Username: "bob"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)

	n, err := h.queries.CountUsers(context.Background())
	r.NoError(err)
	r.Equal(int64(2), n, "both register calls must have persisted")
}

// TestEnvVarBootstrapSeedsFirstUser pins the env-var bootstrap
// convenience: when NEXTCHAPTER_BOOTSTRAP_USERNAME/_PASSWORD are set,
// the operator's account exists in the DB before any HTTP traffic and
// can log in straight away. Crucially, /auth/register stays open after
// bootstrap — the route is unconditionally available now.
func TestEnvVarBootstrapSeedsFirstUser(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	cfg := defaultCfg()
	cfg.BootstrapUsername = "alice"
	cfg.BootstrapPassword = "correct horse battery"
	h := startServer(t, cfg)

	// Pre-condition: the bootstrap user is already persisted before any
	// HTTP request lands.
	preRow, err := h.queries.GetUserByUsername(context.Background(), "alice")
	r.NoError(err, "env-var bootstrap must seed the user on first boot")
	r.Equal("alice", preRow.Username)

	(testRequest{
		Name:           "/auth/login with the bootstrap creds succeeds",
		Method:         http.MethodPost,
		Path:           "/auth/login",
		Body:           models.Credentials{Username: "alice", Password: "correct horse battery"},
		ExpectedStatus: http.StatusOK,
		ExpectedBody:   models.User{Username: "alice"},
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
		ExpectedBody:   models.User{Username: "alice"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)

	// /auth/register stays open after env-var bootstrap: anyone else
	// can still register a separate account.
	(testRequest{
		Name:           "/auth/register still accepts new users after bootstrap",
		Method:         http.MethodPost,
		Path:           "/auth/register",
		Body:           models.Registration{Username: "mallory", Password: "another password"},
		Client:         h.bareClient(), // keep alice's cookie jar untouched
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   models.User{Username: "mallory"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)

	n, err := h.queries.CountUsers(context.Background())
	r.NoError(err)
	r.Equal(int64(2), n, "bootstrap user + /auth/register user")
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
		Body:           models.SeriesNew{Title: "Solo Leveling", Status: constants.StatusReading},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   models.Series{Title: "Solo Leveling", Status: constants.StatusReading, Tags: []string{}},
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
		Body: models.EntryCapture{
			SiteHost:   "reader.example.com",
			SeriesSlug: "solo-leveling",
			SiteTitle:  "Solo Leveling",
			Chapter:    &chapter100,
			URL:        "https://reader.example.com/series/solo-leveling/chapter-100",
			SeriesID:   &seriesID,
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.Entry{
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
		Body: models.EntryCapture{
			SiteHost:   "reader.example.com",
			SeriesSlug: "solo-leveling",
			SiteTitle:  "Solo Leveling",
			Chapter:    &chapter110,
			URL:        "https://reader.example.com/series/solo-leveling/chapter-110.5",
		},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.Entry{
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
		Body: models.EntryCapture{
			SiteHost:   "reader.example.com",
			SeriesSlug: "solo-leveling",
			SiteTitle:  "Solo Leveling",
			Chapter:    &chapter90,
			URL:        "https://reader.example.com/series/solo-leveling/chapter-90",
		},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.Entry{
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
		Body: models.EntryCapture{
			SiteHost:   "comics.example.org",
			SeriesSlug: "solo-leveling-i-alone-level-up",
			SiteTitle:  "Solo Leveling (I Alone Level Up)",
			Chapter:    &chapter105,
			URL:        "https://comics.example.org/comic/solo-leveling-i-alone-level-up/chapter-105",
			SeriesID:   &seriesID,
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.Entry{
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
		ExpectedBody: models.SeriesList{
			Items: []models.SeriesSummary{{
				Series: models.Series{
					Title:  "Solo Leveling",
					Status: constants.StatusReading,
					Tags:   []string{},
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
		Body:           models.SeriesNew{Title: "Solo Leveling"},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   models.Series{Title: "Solo Leveling", Status: constants.StatusReading, Tags: []string{}},
		SentinelPaths:  []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	s1ID := decodeSeriesID(t, s1Body)

	_, s2Body := (testRequest{
		Name:           "seed series 2",
		Method:         http.MethodPost,
		Path:           "/series",
		Body:           models.SeriesNew{Title: "Solo Leveling (continuation)"},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   models.Series{Title: "Solo Leveling (continuation)", Status: constants.StatusReading, Tags: []string{}},
		SentinelPaths:  []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	s2ID := decodeSeriesID(t, s2Body)

	chapter100 := 100.0
	_, eBody := (testRequest{
		Name:   "seed entry on series 1",
		Method: http.MethodPost,
		Path:   "/entries/capture",
		Body: models.EntryCapture{
			SiteHost:   "reader.example.com",
			SeriesSlug: "solo-leveling",
			SiteTitle:  "Solo Leveling",
			Chapter:    &chapter100,
			URL:        "https://reader.example.com/series/solo-leveling/chapter-100",
			SeriesID:   &s1ID,
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.Entry{
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
		Body:           models.EntryPatch{SeriesID: &s2ID},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.Entry{
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
		ExpectedBody: models.SeriesDetail{
			SeriesSummary: models.SeriesSummary{
				Series: models.Series{
					ID:     s2ID,
					Title:  "Solo Leveling (continuation)",
					Status: constants.StatusReading,
					Tags:   []string{},
				},
				HighestChapter: &highest,
				EntryCount:     1,
				LastCapturedAt: nil,
			},
			Entries: []models.Entry{{
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
		Body:           models.NewToken{Label: "extension"},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.APIToken{
			Label:      "extension",
			LastUsedAt: nil,
			ExpiresAt:  nil,
			// Raw (json:"token") is non-deterministic; the sentinel
			// path replaces it with "<token>" on both sides.
			Raw: "<token>",
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
		ExpectedBody:   models.User{Username: "alice"},
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
		ExpectedBody:   models.Health{Status: "ok", Version: "test"},
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
// PATCH — see [models.SeriesPatch].
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
		Body: models.SeriesNew{
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusReading,
			Rating: intPtr(8),
			Notes:  "initial",
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.Series{
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusReading,
			Rating: intPtr(8),
			Notes:  "initial",
			Tags:   []string{},
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
		Body:           models.SeriesPatch{Notes: &notes},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.Series{
			ID:     seriesID,
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusReading,
			Rating: intPtr(8),
			Notes:  "now with thoughts",
			Tags:   []string{},
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
		ExpectedBody: models.Series{
			ID:     seriesID,
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusReading,
			Rating: intPtr(8),
			Notes:  "now with thoughts",
			Tags:   []string{},
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
		Body:           models.SeriesPatch{Status: &status},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.Series{
			ID:     seriesID,
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusCompleted,
			Rating: intPtr(8),
			Notes:  "now with thoughts",
			Tags:   []string{},
		},
		SentinelPaths: []string{"created_at", "updated_at"},
	}).do(t, h)

	finalRow := requireSeriesRow(t, h, seriesID)
	r.Equal(constants.StatusCompleted, finalRow.Status)

	// Tag replacement: PATCH with a non-nil Tags pointer wholesale
	// replaces the tag set. We seed [a,b], then ["c"] replaces it, then
	// a tag-less PATCH must NOT clear "c".
	initial := []string{"a", "b"}
	(testRequest{
		Name:           "seed tags via PATCH",
		Method:         http.MethodPatch,
		Path:           patchPath,
		Body:           models.SeriesPatch{Tags: &initial},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.Series{
			ID:     seriesID,
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusCompleted,
			Rating: intPtr(8),
			Notes:  "now with thoughts",
			Tags:   []string{"a", "b"},
		},
		SentinelPaths: []string{"created_at", "updated_at"},
	}).do(t, h)

	tagsNow, err := h.queries.GetSeriesTags(context.Background(), seriesID)
	r.NoError(err)
	r.Equal([]string{"a", "b"}, tagsNow)

	replacement := []string{"c"}
	(testRequest{
		Name:           "PATCH with a non-nil tags pointer wholesale replaces the set",
		Method:         http.MethodPatch,
		Path:           patchPath,
		Body:           models.SeriesPatch{Tags: &replacement},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.Series{
			ID:     seriesID,
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusCompleted,
			Rating: intPtr(8),
			Notes:  "now with thoughts",
			Tags:   []string{"c"},
		},
		SentinelPaths: []string{"created_at", "updated_at"},
	}).do(t, h)

	tagsNow, err = h.queries.GetSeriesTags(context.Background(), seriesID)
	r.NoError(err)
	r.Equal([]string{"c"}, tagsNow)

	notes2 := "tags untouched"
	(testRequest{
		Name:           "PATCH without tags leaves the existing tag set untouched",
		Method:         http.MethodPatch,
		Path:           patchPath,
		Body:           models.SeriesPatch{Notes: &notes2},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.Series{
			ID:     seriesID,
			Title:  "Omniscient Reader's Viewpoint",
			Status: constants.StatusCompleted,
			Rating: intPtr(8),
			Notes:  "tags untouched",
			Tags:   []string{"c"},
		},
		SentinelPaths: []string{"created_at", "updated_at"},
	}).do(t, h)

	tagsNow, err = h.queries.GetSeriesTags(context.Background(), seriesID)
	r.NoError(err)
	r.Equal([]string{"c"}, tagsNow)

	// Validation: an uppercase tag fails the `tagname` validator.
	(testRequest{
		Name:           "PATCH with an uppercase tag returns 422",
		Method:         http.MethodPatch,
		Path:           patchPath,
		Body:           map[string]any{"tags": []string{"UPPERCASE"}},
		ExpectedStatus: http.StatusUnprocessableEntity,
		ExpectedBody: errorBody{Error: errorPayload{
			Code:    "validation",
			Message: "invalid request",
			Fields:  map[string]string{"tags": "must match ^[a-z0-9][a-z0-9-]{0,31}$"},
		}},
	}).do(t, h)

	// Validation: 17 tags exceeds the `max=16` cap.
	tooMany := make([]string, 17)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("tag-%d", i)
	}
	(testRequest{
		Name:           "PATCH with more than 16 tags returns 422",
		Method:         http.MethodPatch,
		Path:           patchPath,
		Body:           models.SeriesPatch{Tags: &tooMany},
		ExpectedStatus: http.StatusUnprocessableEntity,
		ExpectedBody: errorBody{Error: errorPayload{
			Code:    "validation",
			Message: "invalid request",
			Fields:  map[string]string{"tags": "must be <= 16"},
		}},
	}).do(t, h)
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
		Body:           models.SeriesNew{Title: "Solo Leveling (host-norm)", Status: constants.StatusReading},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   models.Series{Title: "Solo Leveling (host-norm)", Status: constants.StatusReading, Tags: []string{}},
		SentinelPaths:  []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	seriesID := decodeSeriesID(t, seriesBody)

	c1 := 1.0
	url1 := fmt.Sprintf("https://reader.example.com/series/solo-leveling-host-norm/chapter-%d", 1)
	_, body := (testRequest{
		Name:   "first capture normalises SiteHost to lowercase no-www",
		Method: http.MethodPost,
		Path:   "/entries/capture",
		Body: models.EntryCapture{
			SiteHost:   "WWW.reader.example.com",
			SeriesSlug: "solo-leveling-host-norm",
			SiteTitle:  "Solo Leveling",
			Chapter:    &c1,
			URL:        url1,
			SeriesID:   &seriesID,
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.Entry{
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
		Body: models.EntryCapture{
			SiteHost:   "reader.example.com",
			SeriesSlug: "solo-leveling-host-norm",
			SiteTitle:  "Solo Leveling",
			Chapter:    &c2,
			URL:        url2,
		},
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.Entry{
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

// TestUserTagsSeriesAndFiltersByTag walks the tag CRUD surface
// end-to-end: tracking three series with overlapping tag sets, then
// listing them via ?tag=... and checking the AND-semantic filter. The
// final assertions go through the DB to confirm the tag and
// series_tag tables actually hold what the wire response advertised.
func TestUserTagsSeriesAndFiltersByTag(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	aliceUID := aliceID(t, h)

	// Three series under alice: [a,b], [a,c], [b]. Track each via
	// POST /series with the Tags field on the inbound payload.
	_, body1 := (testRequest{
		Name:   "track series with tags [a, b]",
		Method: http.MethodPost,
		Path:   "/series",
		Body: models.SeriesNew{
			Title: "Series One",
			Tags:  []string{"a", "b"},
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.Series{
			Title:  "Series One",
			Status: constants.StatusReading,
			Tags:   []string{"a", "b"},
		},
		SentinelPaths: []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	s1ID := decodeSeriesID(t, body1)

	_, body2 := (testRequest{
		Name:   "track series with tags [a, c]",
		Method: http.MethodPost,
		Path:   "/series",
		Body: models.SeriesNew{
			Title: "Series Two",
			Tags:  []string{"a", "c"},
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.Series{
			Title:  "Series Two",
			Status: constants.StatusReading,
			Tags:   []string{"a", "c"},
		},
		SentinelPaths: []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	s2ID := decodeSeriesID(t, body2)

	_, body3 := (testRequest{
		Name:   "track series with tag [b]",
		Method: http.MethodPost,
		Path:   "/series",
		Body: models.SeriesNew{
			Title: "Series Three",
			Tags:  []string{"b"},
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.Series{
			Title:  "Series Three",
			Status: constants.StatusReading,
			Tags:   []string{"b"},
		},
		SentinelPaths: []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	s3ID := decodeSeriesID(t, body3)

	// Filter by ?tag=a → series one + two (NOT three).
	(testRequest{
		Name:           "GET /series?tag=a returns the two series that carry tag a",
		Method:         http.MethodGet,
		Path:           "/series?tag=a",
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.SeriesList{
			Items: []models.SeriesSummary{
				{
					Series: models.Series{
						ID:     s2ID,
						Title:  "Series Two",
						Status: constants.StatusReading,
						Tags:   []string{"a", "c"},
					},
					HighestChapter: nil,
					EntryCount:     0,
					LastCapturedAt: nil,
				},
				{
					Series: models.Series{
						ID:     s1ID,
						Title:  "Series One",
						Status: constants.StatusReading,
						Tags:   []string{"a", "b"},
					},
					HighestChapter: nil,
					EntryCount:     0,
					LastCapturedAt: nil,
				},
			},
			Total: 2,
		},
		SentinelPaths: []string{
			"items.*.created_at",
			"items.*.updated_at",
		},
	}).do(t, h)

	// Filter by ?tag=a&tag=b → only series one (AND-semantic).
	(testRequest{
		Name:           "GET /series?tag=a&tag=b applies AND semantics",
		Method:         http.MethodGet,
		Path:           "/series?tag=a&tag=b",
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.SeriesList{
			Items: []models.SeriesSummary{{
				Series: models.Series{
					ID:     s1ID,
					Title:  "Series One",
					Status: constants.StatusReading,
					Tags:   []string{"a", "b"},
				},
				HighestChapter: nil,
				EntryCount:     0,
				LastCapturedAt: nil,
			}},
			Total: 1,
		},
		SentinelPaths: []string{
			"items.*.created_at",
			"items.*.updated_at",
		},
	}).do(t, h)

	// Filter by ?tag=nonexistent → empty.
	(testRequest{
		Name:           "GET /series?tag=nonexistent returns an empty page",
		Method:         http.MethodGet,
		Path:           "/series?tag=nonexistent",
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.SeriesList{
			Items: []models.SeriesSummary{},
			Total: 0,
		},
	}).do(t, h)

	// Store-state: the `tag` table holds three rows (a, b, c) under
	// alice; the `series_tag` table holds five links.
	tagRows, err := h.queries.ListTagsByUser(context.Background(), aliceUID)
	r.NoError(err)
	r.Equal([]string{"a", "b", "c"}, tagRows)

	s1Tags, err := h.queries.GetSeriesTags(context.Background(), s1ID)
	r.NoError(err)
	r.Equal([]string{"a", "b"}, s1Tags)
	s2Tags, err := h.queries.GetSeriesTags(context.Background(), s2ID)
	r.NoError(err)
	r.Equal([]string{"a", "c"}, s2Tags)
	s3Tags, err := h.queries.GetSeriesTags(context.Background(), s3ID)
	r.NoError(err)
	r.Equal([]string{"b"}, s3Tags)
}
