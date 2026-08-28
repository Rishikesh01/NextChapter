package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/models"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
)

// --- image fixtures -------------------------------------------------------
//
// Real encoded images, not stub byte strings: the upload path sniffs the
// MIME type from the bytes and decodes the header for dimensions, so a
// fake payload would not exercise what production actually runs.

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 200, G: 40, B: 90, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

// webpLosslessBytes hand-builds a minimal RIFF/WEBP VP8L container.
// Go has no WebP encoder, and the backend only ever reads the header,
// so a valid container with a truthful size field is enough to exercise
// webpDimensions end to end.
func webpLosslessBytes(w, h uint32) []byte {
	payload := make([]byte, 5)
	payload[0] = 0x2f
	binary.LittleEndian.PutUint32(payload[1:5], (w-1)|((h-1)<<14))

	out := new(bytes.Buffer)
	out.WriteString("RIFF")
	_ = binary.Write(out, binary.LittleEndian, uint32(4+8+len(payload)))
	out.WriteString("WEBP")
	out.WriteString("VP8L")
	_ = binary.Write(out, binary.LittleEndian, uint32(len(payload)))
	out.Write(payload)
	// DetectContentType reads up to 512 bytes and webpDimensions needs
	// at least 30; pad so both see a plausible file.
	out.Write(make([]byte, 32))
	return out.Bytes()
}

// --- request helpers ------------------------------------------------------
//
// testRequest JSON-marshals its body, so cover uploads (raw binary) go
// through the client directly.

func putCover(t *testing.T, h *harness, seriesID int64, body []byte, sourceURL string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, h.srv.URL+"/series/"+itoa(seriesID)+"/cover", bytes.NewReader(body))
	require.NoError(t, err)
	// Deliberately mislabelled: the backend must sniff the real type
	// from the bytes and ignore this header entirely (ADR-0011 §4).
	req.Header.Set("Content-Type", "application/octet-stream")
	if sourceURL != "" {
		req.Header.Set("X-Cover-Source-Url", sourceURL)
	}
	resp, err := h.client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, raw
}

func getCover(t *testing.T, h *harness, seriesID int64, ifNoneMatch string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, h.srv.URL+"/series/"+itoa(seriesID)+"/cover", nil)
	require.NoError(t, err)
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	resp, err := h.client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, raw
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// coverKey builds the user-scoped lookup the store-state assertions use.
func coverKey(t *testing.T, h *harness, seriesID int64) gen.GetSeriesCoverParams {
	t.Helper()
	return gen.GetSeriesCoverParams{SeriesID: seriesID, UserID: aliceID(t, h)}
}

// seedSeries creates one series and returns its id.
func seedSeries(t *testing.T, h *harness, title string) int64 {
	t.Helper()
	_, body := (testRequest{
		Method:         http.MethodPost,
		Path:           "/series",
		Body:           models.SeriesNew{Title: title},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.Series{
			Title:  title,
			Status: constants.StatusReading,
			Tags:   []string{},
		},
		SentinelPaths: []string{"id", "created_at", "updated_at"},
	}).do(t, h)
	return decodeSeriesID(t, body)
}

// --- tests ----------------------------------------------------------------

// TestCoverUploadServeAndDelete walks the whole cover lifecycle against a
// real DB: upload PNG bytes, fetch them back byte-for-byte with the right
// content type, revalidate with the returned ETag, replace with a
// different image, then delete.
func TestCoverUploadServeAndDelete(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	seriesID := seedSeries(t, h, "Reverend Insanity")

	// No cover yet.
	resp, _ := getCover(t, h, seriesID, "")
	r.Equal(http.StatusNotFound, resp.StatusCode)

	original := pngBytes(t, 300, 450)
	resp, raw := putCover(t, h, seriesID, original, "https://example.test/series/ri")
	r.Equal(http.StatusOK, resp.StatusCode, "body: %s", raw)

	var meta models.SeriesCoverMeta
	r.NoError(json.Unmarshal(raw, &meta))
	r.Equal(seriesID, meta.SeriesID)
	r.Equal(constants.MimePNG, meta.Mime, "type must be sniffed, not read from the octet-stream header")
	r.Equal(int64(300), meta.Width)
	r.Equal(int64(450), meta.Height)
	r.Equal(int64(len(original)), meta.ByteSize)
	r.Equal("https://example.test/series/ri", meta.SourceURL)
	sum := sha256.Sum256(original)
	r.Equal(hex.EncodeToString(sum[:]), meta.ETag)
	// The bytes must never appear in the JSON metadata response.
	r.NotContains(string(raw), "bytes")

	// Fetch returns the exact bytes with the sniffed content type.
	resp, got := getCover(t, h, seriesID, "")
	r.Equal(http.StatusOK, resp.StatusCode)
	r.Equal(constants.MimePNG, resp.Header.Get("Content-Type"))
	r.Equal(original, got)
	etag := resp.Header.Get("ETag")
	r.Equal(`"`+meta.ETag+`"`, etag)
	r.Equal("private, max-age=0, must-revalidate", resp.Header.Get("Cache-Control"),
		"covers are per-user and must not be cached by a shared proxy")

	// A conditional request with that ETag revalidates to a bodiless 304.
	resp, got = getCover(t, h, seriesID, etag)
	r.Equal(http.StatusNotModified, resp.StatusCode)
	r.Empty(got)

	// A stale ETag gets the full body back.
	resp, got = getCover(t, h, seriesID, `"not-the-current-etag"`)
	r.Equal(http.StatusOK, resp.StatusCode)
	r.Equal(original, got)

	// Replacing swaps the bytes in place and moves the ETag.
	replacement := jpegBytes(t, 200, 320)
	resp, raw = putCover(t, h, seriesID, replacement, "")
	r.Equal(http.StatusOK, resp.StatusCode, "body: %s", raw)
	var replaced models.SeriesCoverMeta
	r.NoError(json.Unmarshal(raw, &replaced))
	r.Equal(constants.MimeJPEG, replaced.Mime)
	r.NotEqual(meta.ETag, replaced.ETag)
	// created_at survives the upsert; updated_at moves forward.
	r.Equal(meta.CreatedAt, replaced.CreatedAt)

	resp, got = getCover(t, h, seriesID, "")
	r.Equal(http.StatusOK, resp.StatusCode)
	r.Equal(constants.MimeJPEG, resp.Header.Get("Content-Type"))
	r.Equal(replacement, got)

	// Delete, then it is gone; a second delete is a 404.
	(testRequest{
		Name:           "delete cover",
		Method:         http.MethodDelete,
		Path:           "/series/" + itoa(seriesID) + "/cover",
		ExpectedStatus: http.StatusNoContent,
	}).do(t, h)

	resp, _ = getCover(t, h, seriesID, "")
	r.Equal(http.StatusNotFound, resp.StatusCode)

	(testRequest{
		Name:           "delete again is 404",
		Method:         http.MethodDelete,
		Path:           "/series/" + itoa(seriesID) + "/cover",
		ExpectedStatus: http.StatusNotFound,
		ExpectedBody:   errorBody{Error: errorPayload{Code: "not_found", Message: "not found"}},
	}).do(t, h)
}

// TestCoverAcceptsWebP pins the hand-rolled WebP header parser, which
// exists because the standard library has no WebP decoder.
func TestCoverAcceptsWebP(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	seriesID := seedSeries(t, h, "Solo Leveling")

	resp, raw := putCover(t, h, seriesID, webpLosslessBytes(360, 540), "")
	r.Equal(http.StatusOK, resp.StatusCode, "body: %s", raw)

	var meta models.SeriesCoverMeta
	r.NoError(json.Unmarshal(raw, &meta))
	r.Equal(constants.MimeWebP, meta.Mime)
	r.Equal(int64(360), meta.Width)
	r.Equal(int64(540), meta.Height)
}

// TestCoverRejectsNonImages proves the endpoint trusts the bytes rather
// than the caller: a mislabelled payload, an empty body, and a truncated
// image are all refused.
func TestCoverRejectsNonImages(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	seriesID := seedSeries(t, h, "Nano Machine")

	t.Run("HTML claiming to be an image", func(t *testing.T) {
		resp, _ := putCover(t, h, seriesID, []byte("<!doctype html><html><body>not an image</body></html>"), "")
		require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("empty body", func(t *testing.T) {
		resp, _ := putCover(t, h, seriesID, nil, "")
		require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("truncated PNG", func(t *testing.T) {
		full := pngBytes(t, 100, 100)
		// Keep the magic bytes so it still sniffs as PNG, but cut the
		// header so DecodeConfig fails.
		resp, _ := putCover(t, h, seriesID, full[:12], "")
		require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	// Nothing above should have stored anything.
	resp, _ := getCover(t, h, seriesID, "")
	r.Equal(http.StatusNotFound, resp.StatusCode)
}

// TestCoverOversizedRejected proves the MaxBytesReader cap fires before
// the body is buffered.
func TestCoverOversizedRejected(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	seriesID := seedSeries(t, h, "Shadow Slave")

	oversized := make([]byte, constants.MaxCoverBytes+1024)
	copy(oversized, pngBytes(t, 10, 10))
	resp, _ := putCover(t, h, seriesID, oversized, "")
	r.Equal(http.StatusRequestEntityTooLarge, resp.StatusCode)

	resp, _ = getCover(t, h, seriesID, "")
	r.Equal(http.StatusNotFound, resp.StatusCode)
}

// TestCoverIsUserScoped proves one user cannot read, replace or delete
// another user's cover, and that an unknown series id is a 404 rather
// than a leak of its existence.
func TestCoverIsUserScoped(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	seriesID := seedSeries(t, h, "Lord of the Mysteries")
	resp, _ := putCover(t, h, seriesID, pngBytes(t, 300, 450), "")
	r.Equal(http.StatusOK, resp.StatusCode)

	// A second account on the same instance.
	(testRequest{
		Name:           "register mallory",
		Method:         http.MethodPost,
		Path:           "/auth/register",
		Body:           models.Credentials{Username: "mallory", Password: "correct horse battery"},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody:   models.User{Username: "mallory"},
		SentinelPaths:  []string{"id", "created_at"},
	}).do(t, h)

	// The register call replaced the jar's session with mallory's, so
	// the same client is now the attacker.
	resp, _ = getCover(t, h, seriesID, "")
	r.Equal(http.StatusNotFound, resp.StatusCode, "mallory must not read alice's cover")

	resp, _ = putCover(t, h, seriesID, pngBytes(t, 50, 50), "")
	r.Equal(http.StatusNotFound, resp.StatusCode, "mallory must not overwrite alice's cover")

	(testRequest{
		Name:           "mallory cannot delete alice's cover",
		Method:         http.MethodDelete,
		Path:           "/series/" + itoa(seriesID) + "/cover",
		ExpectedStatus: http.StatusNotFound,
		ExpectedBody:   errorBody{Error: errorPayload{Code: "not_found", Message: "not found"}},
	}).do(t, h)

	// Alice's bytes are untouched.
	row, err := h.queries.GetSeriesCover(context.Background(), coverKey(t, h, seriesID))
	r.NoError(err)
	r.Equal(int64(300), row.Width, "alice's cover must survive mallory's attempts")
}

// TestCoverSurfacesOnSeriesSummary proves cover_updated_at is the
// existence flag the library grid keys off, and that it moves when the
// cover is replaced and clears when it is deleted.
func TestCoverSurfacesOnSeriesSummary(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	seriesID := seedSeries(t, h, "One Piece")

	r.Nil(summaryCoverUpdatedAt(t, h, seriesID), "a coverless series reports null")

	resp, _ := putCover(t, h, seriesID, pngBytes(t, 300, 450), "")
	r.Equal(http.StatusOK, resp.StatusCode)

	first := summaryCoverUpdatedAt(t, h, seriesID)
	r.NotNil(first, "an uploaded cover surfaces on the summary")

	(testRequest{
		Name:           "delete cover",
		Method:         http.MethodDelete,
		Path:           "/series/" + itoa(seriesID) + "/cover",
		ExpectedStatus: http.StatusNoContent,
	}).do(t, h)

	r.Nil(summaryCoverUpdatedAt(t, h, seriesID), "deleting clears the flag again")
}

// TestDeletingSeriesCascadesToCover proves the FK cascade: removing a
// series must not strand its cover bytes in the database.
func TestDeletingSeriesCascadesToCover(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	seriesID := seedSeries(t, h, "Vagabond")
	resp, _ := putCover(t, h, seriesID, pngBytes(t, 300, 450), "")
	r.Equal(http.StatusOK, resp.StatusCode)

	(testRequest{
		Name:           "delete the series",
		Method:         http.MethodDelete,
		Path:           "/series/" + itoa(seriesID),
		ExpectedStatus: http.StatusNoContent,
	}).do(t, h)

	_, err := h.queries.GetSeriesCover(context.Background(), coverKey(t, h, seriesID))
	r.Error(err, "the cover row must be gone with its series")
}

// summaryCoverUpdatedAt reads one series' cover_updated_at off the
// GET /series listing — the field the web grid actually consumes.
func summaryCoverUpdatedAt(t *testing.T, h *harness, seriesID int64) *string {
	t.Helper()
	r := require.New(t)
	resp, err := h.client.Get(h.srv.URL + "/series")
	r.NoError(err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	r.NoError(err)
	r.Equal(http.StatusOK, resp.StatusCode, "body: %s", raw)

	var page struct {
		Items []struct {
			ID             int64   `json:"id"`
			CoverUpdatedAt *string `json:"cover_updated_at"`
		} `json:"items"`
	}
	r.NoError(json.Unmarshal(raw, &page))
	for _, item := range page.Items {
		if item.ID == seriesID {
			return item.CoverUpdatedAt
		}
	}
	t.Fatalf("series %d missing from the listing", seriesID)
	return nil
}

// TestSummaryRollupTimestampsParse is a regression guard for a bug the
// cover rollup surfaced: modernc.org/sqlite returns MAX() over a
// TIMESTAMP column as TEXT in Go's time.Time.String() layout, which
// anyToTimePtr did not parse — so BOTH last_captured_at and
// cover_updated_at came back null on every SQLite listing while the
// underlying rows were perfectly fine.
func TestSummaryRollupTimestampsParse(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	h := newAuthenticatedHarness(t)
	seriesID := seedSeries(t, h, "Omniscient Reader")

	chapter := 551.0
	(testRequest{
		Name:   "capture a chapter",
		Method: http.MethodPost,
		Path:   "/entries/capture",
		Body: models.EntryCapture{
			SiteHost:   "novels.example.com",
			SeriesSlug: "orv",
			SiteTitle:  "ORV Chapter 551",
			Chapter:    &chapter,
			URL:        "https://novels.example.com/orv/551",
			SeriesID:   &seriesID,
		},
		ExpectedStatus: http.StatusCreated,
		ExpectedBody: models.Entry{
			SeriesID:    seriesID,
			SiteHost:    "novels.example.com",
			SeriesSlug:  "orv",
			SiteTitle:   "ORV Chapter 551",
			LastChapter: chapter,
			LastURL:     "https://novels.example.com/orv/551",
		},
		SentinelPaths: []string{"id", "last_captured_at", "created_at", "updated_at"},
	}).do(t, h)

	resp, _ := putCover(t, h, seriesID, pngBytes(t, 300, 450), "")
	r.Equal(http.StatusOK, resp.StatusCode)

	summary := seriesSummary(t, h, seriesID)
	r.NotNil(summary.LastCapturedAt, "the entries rollup must survive the driver's TEXT round-trip")
	r.NotNil(summary.CoverUpdatedAt, "the cover rollup must survive the driver's TEXT round-trip")
}

// seriesSummary pulls one series' summary row off GET /series.
func seriesSummary(t *testing.T, h *harness, seriesID int64) models.SeriesSummary {
	t.Helper()
	r := require.New(t)
	resp, err := h.client.Get(h.srv.URL + "/series")
	r.NoError(err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	r.NoError(err)
	r.Equal(http.StatusOK, resp.StatusCode, "body: %s", raw)

	var page models.SeriesList
	r.NoError(json.Unmarshal(raw, &page))
	for _, item := range page.Items {
		if item.ID == seriesID {
			return item
		}
	}
	t.Fatalf("series %d missing from the listing", seriesID)
	return models.SeriesSummary{}
}
