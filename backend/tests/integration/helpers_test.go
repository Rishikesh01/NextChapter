// Package integration_test exercises the NextChapter HTTP API end to
// end against a real database. It does not mock the DB; sqlite is
// pure-Go and Postgres runs via testcontainers-go, so both real
// driver paths are exercised.
//
// Default: SQLite tempfile per test (fast, no Docker required).
// Opt-in: set NEXTCHAPTER_TEST_POSTGRES=1 to run against a Postgres
// container started once per test package in TestMain. When the env
// var is set the harness uses Postgres for *every* test in this
// package; each test gets its own freshly-created database in the
// shared container for full isolation.
package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/internal/auth"
	"github.com/enable-it/nextchapter/backend/internal/config"
	"github.com/enable-it/nextchapter/backend/internal/entries"
	"github.com/enable-it/nextchapter/backend/internal/httpapi"
	"github.com/enable-it/nextchapter/backend/internal/models"
	"github.com/enable-it/nextchapter/backend/internal/series"
	"github.com/enable-it/nextchapter/backend/internal/sites"
	"github.com/enable-it/nextchapter/backend/internal/store"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
	"github.com/enable-it/nextchapter/backend/internal/users"
)

// --- dual-dialect harness wiring -----------------------------------------
//
// pgContainerURL is the admin connection string for the Postgres
// container started by TestMain when NEXTCHAPTER_TEST_POSTGRES=1. It
// stays empty for SQLite-only runs and startServer falls back to a
// tempfile SQLite DB.
//
// Each test that uses Postgres gets its own CREATE DATABASE on the
// shared cluster (the cluster start is the expensive part; CREATE
// DATABASE is cheap), so tests stay isolated without paying the
// per-test container startup cost.
var (
	pgContainerURL  string
	pgDatabaseCount atomic.Int64
)

// useTestPostgres reports whether the integration test package is
// running in dual-dialect mode (Postgres container active).
func useTestPostgres() bool {
	v := os.Getenv("NEXTCHAPTER_TEST_POSTGRES")
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// TestMain bootstraps the Postgres container exactly once for the
// package when NEXTCHAPTER_TEST_POSTGRES=1. The container is torn
// down on exit. When the env var is unset the test process runs
// SQLite-only and TestMain is a no-op shell around m.Run().
func TestMain(m *testing.M) {
	if !useTestPostgres() {
		os.Exit(m.Run())
	}
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("nextchapter_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres testcontainer start: %v\n", err)
		os.Exit(1)
	}
	u, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = c.Terminate(ctx)
		fmt.Fprintf(os.Stderr, "postgres connection string: %v\n", err)
		os.Exit(1)
	}
	pgContainerURL = u
	code := m.Run()
	if err := c.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "postgres terminate: %v\n", err)
	}
	os.Exit(code)
}

// freshPostgresDatabaseURL creates a new isolated database in the
// shared container and returns its connection URL. The database is
// dropped via t.Cleanup so a failed test doesn't pollute the cluster.
func freshPostgresDatabaseURL(t *testing.T) string {
	t.Helper()
	r := require.New(t)

	// Atomic counter + random suffix prevents collisions across
	// parallel subtests in the same binary.
	idx := pgDatabaseCount.Add(1)
	buf := make([]byte, 4)
	_, err := rand.Read(buf)
	r.NoError(err)
	name := fmt.Sprintf("ncx_%d_%s", idx, hex.EncodeToString(buf))

	admin, err := sql.Open("pgx", pgContainerURL)
	r.NoError(err)
	defer func() {
		if err := admin.Close(); err != nil {
			t.Logf("close admin db: %v", err)
		}
	}()
	// #nosec G201 — `name` is generated locally from a counter + crypto
	// random bytes; identifiers can't be parameterised in CREATE
	// DATABASE.
	_, err = admin.ExecContext(context.Background(), "CREATE DATABASE "+name)
	r.NoError(err)

	t.Cleanup(func() {
		dropAdmin, err := sql.Open("pgx", pgContainerURL)
		if err != nil {
			t.Logf("open admin db for cleanup: %v", err)
			return
		}
		defer func() { _ = dropAdmin.Close() }()
		// FORCE drops any active connections from the test process
		// before the underlying database is removed.
		// #nosec G201 — same identifier-not-parameterisable argument.
		if _, err := dropAdmin.ExecContext(context.Background(),
			"DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Logf("drop database %s: %v", name, err)
		}
	})

	// Rewrite the path component of the admin URL to point at the new
	// database. ConnectionString returns something like
	// postgres://test:test@host:port/nextchapter_test?sslmode=disable
	// — we swap the path segment for the per-test name.
	u, err := url.Parse(pgContainerURL)
	r.NoError(err)
	u.Path = "/" + name
	return u.String()
}

// harness is the integration test fixture. Tests get one of these from
// newHarness / startServer and use it to send requests via testRequest
// and to query the underlying store for data-layer assertions.
//
// Store-state assertions go through h.queries for the canonical
// sqlc-generated lookup signatures the integration tests have always
// used; the post-refactor repositories wrap the same *gen.Queries, so
// the test-side checks observe the exact same rows the services do.
type harness struct {
	srv     *httptest.Server
	client  *http.Client
	db      *sql.DB
	queries *gen.Queries
}

// startServer spins up an httptest.Server backed by a fresh per-test
// database, plus a cookie-jar-equipped *http.Client. Cleanup is
// registered via t.Cleanup.
//
// The default integration suite is SQLite-canonical because the
// store-state assertions reach into the sqlc-generated SQLite
// `Queries` directly (see h.queries). The dual-dialect coverage lives
// in TestPostgresSmoke, which builds its own freshly-created Postgres
// database via [freshPostgresDatabaseURL] and passes the URL through
// cfg.DatabaseURL; this function honours any non-default
// (non-sqlite://./nextchapter.db) URL the caller has already set.
func startServer(t *testing.T, cfg config.Config) *harness {
	t.Helper()
	r := require.New(t)
	ctx := context.Background()
	// config.Default() pre-populates DatabaseURL with the production
	// default path; treat that sentinel as "no caller override" and
	// substitute a per-test tempfile. Any other value (e.g. a Postgres
	// URL provisioned by freshPostgresDatabaseURL) is preserved.
	if cfg.DatabaseURL == "" || cfg.DatabaseURL == config.Default().DatabaseURL {
		dbPath := filepath.Join(t.TempDir(), "nextchapter.db")
		cfg.DatabaseURL = "sqlite://" + dbPath
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:0"
	}
	if cfg.Version == "" {
		cfg.Version = "test"
	}
	r.NoError(cfg.Validate())

	db, err := store.Open(ctx, cfg.DatabaseURL)
	r.NoError(err)
	r.NoError(store.Migrate(ctx, db, cfg.DatabaseURL))

	dialect, err := store.DialectFor(cfg.DatabaseURL)
	r.NoError(err)

	// h.queries is the SQLite-flavoured sqlc handle used by the
	// integration suite for store-state assertions. It is nil for
	// Postgres harnesses; the Postgres smoke test uses wire-shape
	// assertions only.
	var q *gen.Queries
	if dialect == "sqlite3" {
		q = gen.New(db)
	}
	userRepo, err := users.NewRepository(dialect, db)
	r.NoError(err)
	usrSvc := users.NewService(userRepo, zap.NewNop())
	authRepo, err := auth.NewRepository(dialect, db)
	r.NoError(err)
	authSvc := auth.NewService(authRepo, userRepo, zap.NewNop())
	entRepo, err := entries.NewRepository(dialect, db)
	r.NoError(err)
	entSvc := entries.NewService(entRepo, zap.NewNop())
	srsRepo, err := series.NewRepository(dialect, db)
	r.NoError(err)
	srsSvc := series.NewService(srsRepo, entSvc, zap.NewNop())
	stsRepo, err := sites.NewRepository(dialect, db)
	r.NoError(err)
	stsSvc := sites.NewService(stsRepo, zap.NewNop())

	if cfg.HasBootstrap() {
		// Env-var bootstrap pre-seeds the operator's account. The
		// /auth/register route stays open regardless; tests build a
		// fresh tempfile DB per harness so the first call always
		// succeeds.
		u, err := usrSvc.Register(ctx, models.Registration{
			Username: cfg.BootstrapUsername,
			Password: cfg.BootstrapPassword,
		})
		r.NoError(err)
		r.NoError(stsSvc.SeedSiteRulesForUser(ctx, u.ID))
	}

	engine := httpapi.New(httpapi.Deps{
		Users:   usrSvc,
		Auth:    authSvc,
		Series:  srsSvc,
		Entries: entSvc,
		Sites:   stsSvc,
		Logger:  zap.NewNop(),
		Version: cfg.Version,
	})

	srv := httptest.NewServer(engine)
	jar, err := cookiejar.New(nil)
	r.NoError(err)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}

	t.Cleanup(func() {
		srv.Close()
		if err := db.Close(); err != nil {
			t.Logf("close db: %v", err)
		}
	})

	return &harness{srv: srv, client: client, db: db, queries: q}
}

// newHarness is the default fixture: open-registration config, no
// bootstrap user. Use newAuthenticatedHarness when you want a logged
// in alice.
func newHarness(t *testing.T) *harness {
	t.Helper()
	return startServer(t, defaultCfg())
}

// bareClient returns a fresh *http.Client with no cookie jar. Used for
// API-token Bearer scenarios where we want to verify auth without
// session-cookie spillover.
func (h *harness) bareClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}

// defaultCfg returns the open-registration config used by tests that
// drive the /auth/register window.
func defaultCfg() config.Config {
	cfg := config.Default()
	cfg.Version = "test"
	return cfg
}

// bootstrappedCfg returns a config that uses env-var bootstrap. The
// resulting harness already has user "alice" in the DB.
func bootstrappedCfg() config.Config {
	cfg := config.Default()
	cfg.BootstrapUsername = "alice"
	cfg.BootstrapPassword = "correct horse battery"
	return cfg
}

// newAuthenticatedHarness returns a harness whose default client is
// already logged in as the bootstrap user "alice".
func newAuthenticatedHarness(t *testing.T) *harness {
	t.Helper()
	h := startServer(t, bootstrappedCfg())
	(testRequest{
		Method:         http.MethodPost,
		Path:           "/auth/login",
		Body:           models.Credentials{Username: "alice", Password: "correct horse battery"},
		ExpectedStatus: http.StatusOK,
		ExpectedBody:   models.User{Username: "alice"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)
	return h
}

// aliceID looks up the bootstrap user's row id. Cached behind a query
// since tests need it for store-state assertions on user-scoped rows.
func aliceID(t *testing.T, h *harness) int64 {
	t.Helper()
	r := require.New(t)
	row, err := h.queries.GetUserByUsername(context.Background(), "alice")
	r.NoError(err)
	return row.ID
}

// requireSeriesRow fetches a series row scoped to alice. Convenience
// wrapper for the common store-state check.
func requireSeriesRow(t *testing.T, h *harness, seriesID int64) gen.Series {
	t.Helper()
	r := require.New(t)
	row, err := h.queries.GetSeriesByID(context.Background(), gen.GetSeriesByIDParams{
		ID:     seriesID,
		UserID: aliceID(t, h),
	})
	r.NoError(err)
	return row
}

// parseURL is a tiny convenience: cookie jar lookups want a *url.URL,
// not a string, and tests want a require-style failure.
func parseURL(t *testing.T, s string) *url.URL {
	t.Helper()
	r := require.New(t)
	u, err := url.Parse(s)
	r.NoError(err)
	return u
}

// decodeEntryID extracts the id of an entry from a response body so
// tests can build follow-up paths like /entries/{id}. Used solely for
// id capture — never for assertions. Body assertions go through
// testRequest.ExpectedBody.
func decodeEntryID(t *testing.T, body []byte) int64 {
	t.Helper()
	r := require.New(t)
	var v struct {
		ID int64 `json:"id"`
	}
	r.NoError(json.Unmarshal(body, &v))
	return v.ID
}

// decodeSeriesID extracts the id of a series. Same justification as
// decodeEntryID: id-capture only.
func decodeSeriesID(t *testing.T, body []byte) int64 {
	t.Helper()
	r := require.New(t)
	var v struct {
		ID int64 `json:"id"`
	}
	r.NoError(json.Unmarshal(body, &v))
	return v.ID
}

// decodeMintedToken extracts the id and raw plaintext token from a
// POST /auth/tokens response. The plaintext is needed for the Bearer
// flow; the id is needed to build the /auth/tokens/{id} delete path.
// Body shape assertions still go through testRequest.ExpectedBody.
func decodeMintedToken(t *testing.T, body []byte) (id int64, token string) {
	t.Helper()
	r := require.New(t)
	var v struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	r.NoError(json.Unmarshal(body, &v))
	return v.ID, v.Token
}

// testRequest is the canonical way an integration test issues an HTTP
// request. Every test instantiates one of these and calls do — no
// ad-hoc http.NewRequest or status / body assertions in test bodies.
//
// Field semantics:
//   - Name: short label used to wrap the request in t.Run(Name, ...).
//     Set this whenever the enclosing test issues more than one
//     request; leave it empty for single-request tests so the runner
//     doesn't add a noisy extra layer in -v output. When Name is
//     empty, do runs inline on the passed *testing.T.
//   - Body / ExpectedBody: typed values from the [internal/models]
//     package (e.g. [models.SeriesNew] for requests, [models.Series]
//     for responses). Marshalled to JSON for both wire payload and
//     JSONEq comparison.
//   - ExpectedBody == nil: assert the response body is empty
//     (e.g. 204 No Content).
//   - SentinelPaths: dotted paths whose values in the actual response
//     are non-deterministic (ids, timestamps, raw tokens) and should
//     be replaced with literal "<segment>" sentinels before JSONEq.
//     The same sentinels must appear in ExpectedBody.
type testRequest struct {
	Name    string // names the subtest; required when used inside a test that has more than one request
	Method  string
	Path    string // absolute path, e.g. "/auth/me"; the harness URL is prefixed
	Body    any
	Headers http.Header
	Cookies []*http.Cookie
	Client  *http.Client // override harness.client (e.g. for bareClient bearer tests)

	ExpectedStatus int
	ExpectedBody   any
	SentinelPaths  []string
}

// do runs the request inside t.Run(r.Name, ...) and asserts status + body.
// Returns (response, raw body) so the test can pull cookies or capture ids.
//
// If r.Name == "", do does NOT call t.Run — it runs inline on the passed t.
// That mode is for the "one request per test function" case (e.g. the
// pre-condition setup at the top of a use-case test, or a test that issues
// exactly one HTTP call).
func (r testRequest) do(t *testing.T, h *harness) (*http.Response, []byte) {
	t.Helper()
	if r.Name == "" {
		return r.run(t, h)
	}
	var (
		resp *http.Response
		body []byte
	)
	t.Run(r.Name, func(t *testing.T) {
		resp, body = r.run(t, h)
	})
	return resp, body
}

// run does the actual work; do calls it either inline or inside t.Run.
// It asserts status code, asserts full response body via JSONEq (with
// sentinels applied to the actual body), and returns the response plus
// raw body bytes so callers can extract cookies or follow-up ids. The
// response body has already been drained and closed by the time run
// returns.
func (r testRequest) run(t *testing.T, h *harness) (*http.Response, []byte) {
	t.Helper()
	rq := require.New(t)

	var bodyReader io.Reader
	if r.Body != nil {
		raw, err := json.Marshal(r.Body)
		rq.NoError(err, "marshal request body for %s %s", r.Method, r.Path)
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(r.Method, h.srv.URL+r.Path, bodyReader)
	rq.NoError(err, "build request for %s %s", r.Method, r.Path)
	if r.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range r.Headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	for _, ck := range r.Cookies {
		req.AddCookie(ck)
	}

	client := r.Client
	if client == nil {
		client = h.client
	}
	resp, err := client.Do(req)
	rq.NoError(err, "send request for %s %s", r.Method, r.Path)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	rq.NoError(err, "read response body for %s %s", r.Method, r.Path)

	rq.Equal(r.ExpectedStatus, resp.StatusCode,
		"status code mismatch for %s %s; body=%s", r.Method, r.Path, string(body))

	if r.ExpectedBody == nil {
		rq.Empty(body, "expected empty body for %s %s", r.Method, r.Path)
		return resp, body
	}

	expectedRaw, err := json.Marshal(r.ExpectedBody)
	rq.NoError(err, "marshal expected body for %s %s", r.Method, r.Path)
	// Sentinel both sides on the same paths. The expected DTO leaves
	// id/timestamps as zero values; this gives those fields the same
	// "<segment>" sentinel on both sides so JSONEq compares the
	// deterministic portion of the body exhaustively.
	expectedJSON := normaliseJSON(t, expectedRaw, r.SentinelPaths...)
	actualJSON := normaliseJSON(t, body, r.SentinelPaths...)
	rq.JSONEq(expectedJSON, actualJSON,
		"body mismatch for %s %s", r.Method, r.Path)

	return resp, body
}

// normaliseJSON unmarshals raw into a generic structure, replaces any
// value at one of the given dotted paths with a sentinel ("<X>"), and
// returns the re-marshalled JSON. Used to keep require.JSONEq
// comparisons exhaustive while ignoring non-deterministic fields
// (ids, timestamps).
//
// Supported path types:
//   - top-level field: "id", "created_at"
//   - nested field via dot: "error.message"
//   - array element: "items.*.id" applies to every element in items.
func normaliseJSON(t *testing.T, raw []byte, paths ...string) string {
	t.Helper()
	r := require.New(t)
	var v any
	r.NoError(json.Unmarshal(raw, &v))
	for _, p := range paths {
		applyPath(v, splitPath(p), "<"+lastSegment(p)+">")
	}
	out, err := json.Marshal(v)
	r.NoError(err)
	return string(out)
}

func splitPath(p string) []string {
	out := []string{}
	cur := ""
	for _, ch := range p {
		if ch == '.' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(ch)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func lastSegment(p string) string {
	parts := splitPath(p)
	if len(parts) == 0 {
		return "x"
	}
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "*" {
			return parts[i]
		}
	}
	return "x"
}

func applyPath(v any, path []string, sentinel string) {
	if len(path) == 0 {
		return
	}
	head, rest := path[0], path[1:]
	switch x := v.(type) {
	case map[string]any:
		if len(rest) == 0 {
			// Only sentinel a present, non-null value. Nulls are
			// deterministic ("the field was unset") and the test should
			// assert the literal null.
			if cur, ok := x[head]; ok && cur != nil {
				x[head] = sentinel
			}
			return
		}
		if child, ok := x[head]; ok {
			applyPath(child, rest, sentinel)
		}
	case []any:
		if head != "*" {
			return
		}
		for _, item := range x {
			applyPath(item, rest, sentinel)
		}
	}
}
