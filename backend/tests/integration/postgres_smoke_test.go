// Postgres smoke test for the dual-engine wiring. This file exists to
// prove the Postgres driver, sqlc-generated `pg` package, and
// dialect-aware repository factories all wire up against a real
// Postgres server.
//
// The SQLite suite carries the depth of the integration coverage
// (every wire-shape, error-envelope, and store-state assertion); this
// file's job is solely to catch dialect-specific regressions in the
// Postgres half of the stack. The test is gated on
// NEXTCHAPTER_TEST_POSTGRES=1; without that env var (the default in
// the SQLite-only CI job and on a developer machine without Docker)
// the test is skipped, so the SQLite suite still runs unchanged.
package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/enable-it/nextchapter/backend/internal/config"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// TestPostgresSmoke exercises the main flows against a real Postgres
// instance. Coverage is intentionally shallow — depth lives in the
// SQLite suite — but the flows touch every domain (users, auth,
// series, entries, sites) so the dialect-specific wiring of all four
// repositories is verified end-to-end.
func TestPostgresSmoke(t *testing.T) {
	if !useTestPostgres() {
		t.Skip("NEXTCHAPTER_TEST_POSTGRES not set; skipping Postgres smoke test")
	}
	r := require.New(t)

	cfg := config.Default()
	cfg.Version = "test-pg"
	cfg.DatabaseURL = freshPostgresDatabaseURL(t)
	h := startServer(t, cfg)

	// 1. /auth/register opens the account on a fresh DB.
	(testRequest{
		Name:           "register opens a new account",
		Method:         http.MethodPost,
		Path:           "/auth/register",
		Body:           models.Registration{Username: "alice", Password: "correct horse battery"},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   models.User{Username: "alice"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)

	// 2. /auth/me confirms the session cookie carries through.
	(testRequest{
		Name:           "session cookie powers /auth/me",
		Method:         http.MethodGet,
		Path:           "/auth/me",
		ExpectedStatus: http.StatusOK,
		ExpectedBody:   models.User{Username: "alice"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)

	// 3. POST /series adds a series row.
	_, seriesBody := (testRequest{
		Name:           "track a series",
		Method:         http.MethodPost,
		Path:           "/series",
		Body:           models.SeriesNew{Title: "Solo Leveling", Status: "reading"},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   models.Series{Title: "Solo Leveling", Status: "reading", Tags: []string{}},
		SentinelPaths:  []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	seriesID := decodeSeriesID(t, seriesBody)
	r.NotZero(seriesID)

	// 4. POST /entries/capture captures a chapter against that series.
	chapter := 12.0
	(testRequest{
		Name:   "capture a chapter",
		Method: http.MethodPost,
		Path:   "/entries/capture",
		Body: models.EntryCapture{
			SeriesID:   &seriesID,
			SiteHost:   "archive.example.net",
			SeriesSlug: "solo-leveling",
			SiteTitle:  "Solo Leveling",
			Chapter:    &chapter,
			URL:        "https://archive.example.net/solo-leveling/chapter-12",
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.Entry{
			SeriesID:    seriesID,
			SiteHost:    "archive.example.net",
			SeriesSlug:  "solo-leveling",
			SiteTitle:   "Solo Leveling",
			LastChapter: 12,
			LastURL:     "https://archive.example.net/solo-leveling/chapter-12",
		},
		SentinelPaths: []string{"id", "last_captured_at", "created_at", "updated_at"},
	}).do(t, h)

	// 5. GET /series — list endpoint exercises the rollup correlated
	//    subqueries (highest_chapter, entry_count, rollup_last_captured_at)
	//    which is the Postgres-flavoured path most likely to drift if
	//    the dialect-specific casts go wrong.
	highestChapter := 12.0
	lastCaptured := time.Unix(0, 0) // placeholder; the sentinel takes over before JSONEq
	(testRequest{
		Name:           "list series shows the rollup",
		Method:         http.MethodGet,
		Path:           "/series",
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.SeriesList{
			Items: []models.SeriesSummary{
				{
					Series: models.Series{
						Title:  "Solo Leveling",
						Status: "reading",
						Tags:   []string{},
					},
					HighestChapter: &highestChapter,
					EntryCount:     1,
					LastCapturedAt: &lastCaptured,
				},
			},
			Total: 1,
		},
		SentinelPaths: []string{
			"items.*.id",
			"items.*.created_at",
			"items.*.updated_at",
			"items.*.last_captured_at",
		},
	}).do(t, h)

	// 6. GET /entries — list endpoint round-trips the entry row.
	(testRequest{
		Name:           "list entries",
		Method:         http.MethodGet,
		Path:           "/entries",
		ExpectedStatus: http.StatusOK,
		ExpectedBody: models.EntryList{
			Items: []models.Entry{
				{
					SeriesID:    seriesID,
					SiteHost:    "archive.example.net",
					SeriesSlug:  "solo-leveling",
					SiteTitle:   "Solo Leveling",
					LastChapter: 12,
					LastURL:     "https://archive.example.net/solo-leveling/chapter-12",
				},
			},
			Total: 1,
		},
		SentinelPaths: []string{
			"items.*.id",
			"items.*.last_captured_at",
			"items.*.created_at",
			"items.*.updated_at",
		},
	}).do(t, h)

	// 7. POST /sites/rules exercises the sites domain's Postgres path.
	(testRequest{
		Name:   "create a site rule",
		Method: http.MethodPost,
		Path:   "/sites/rules",
		Body: models.SiteRuleNew{
			Host:                "panels.example.net",
			ChapterURLRegex:     `^https://panels.example.net/title/(?P<slug>[^/]+)/(?P<chapter>\d+)`,
			SlugCaptureGroup:    "slug",
			ChapterCaptureGroup: "chapter",
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.SiteRule{
			Host:                "panels.example.net",
			ChapterURLRegex:     `^https://panels.example.net/title/(?P<slug>[^/]+)/(?P<chapter>\d+)`,
			SlugCaptureGroup:    "slug",
			ChapterCaptureGroup: "chapter",
		},
		SentinelPaths: []string{"id", "user_id", "created_at", "updated_at"},
	}).do(t, h)

	// 8. Sanity-check the DB is what we think it is via a plain
	//    QueryRowContext — not for assertion depth, but as a tripwire
	//    against the wrong pool getting bound to the wrong test.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var dbName string
	r.NoError(h.db.QueryRowContext(ctx, "SELECT current_database()").Scan(&dbName))
	r.NotEmpty(dbName)
}
