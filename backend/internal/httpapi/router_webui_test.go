package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/internal/auth"
)

// The NoRoute discrimination rule (ADR-0010 §4): browser-shaped GETs get the
// embedded SPA (exact file or the index fallback), API-shaped paths and
// non-GETs keep the JSON not_found envelope, and a nil WebUI preserves the
// pre-SPA behavior everywhere. The auth service is a repo-less shell only so
// route registration succeeds — NoRoute requests never invoke any service.

const indexBody = "<!doctype html><title>t</title>spa-index"

func webuiEngine(t *testing.T, withUI bool) http.Handler {
	t.Helper()
	d := Deps{
		Auth:    auth.NewService(nil, nil, zap.NewNop()),
		Logger:  zap.NewNop(),
		Version: "test",
	}
	if withUI {
		d.WebUI = fstest.MapFS{
			"index.html":            {Data: []byte(indexBody)},
			"assets/app-abc123.js":  {Data: []byte("console.log('app')")},
			"assets/app-abc123.css": {Data: []byte("body{}")},
		}
	}
	return New(d)
}

func doGet(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func assertEnvelope(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not the JSON envelope: %v (%q)", err, w.Body.String())
	}
	if body.Error.Code != "not_found" {
		t.Fatalf("error.code = %q, want not_found", body.Error.Code)
	}
}

func TestWebUIServesIndexAtRoot(t *testing.T) {
	w := doGet(t, webuiEngine(t, true), http.MethodGet, "/")
	if w.Code != http.StatusOK || w.Body.String() != indexBody {
		t.Fatalf("GET / = %d %q, want 200 index", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", cc)
	}
}

func TestWebUIServesHashedAssetImmutable(t *testing.T) {
	w := doGet(t, webuiEngine(t, true), http.MethodGet, "/assets/app-abc123.js")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "console.log") {
		t.Fatalf("asset = %d %q", w.Code, w.Body.String())
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("Cache-Control = %q, want immutable", cc)
	}
}

func TestWebUIFallsBackToIndexForClientRoutes(t *testing.T) {
	for _, path := range []string{"/library", "/rules", "/settings", "/some/deep/route"} {
		w := doGet(t, webuiEngine(t, true), http.MethodGet, path)
		if w.Code != http.StatusOK || w.Body.String() != indexBody {
			t.Fatalf("GET %s = %d %q, want the index fallback", path, w.Code, w.Body.String())
		}
	}
}

func TestWebUIKeepsEnvelopeForAPIPrefixedPaths(t *testing.T) {
	// "/swagger" itself 301s to "/swagger/" via gin's trailing-slash
	// redirect (pre-existing; the route is /swagger/*any), so the prefix
	// rule is pinned through paths that actually reach NoRoute.
	for _, path := range []string{"/auth/nonexistent", "/series/1/extra/deep", "/entries/x", "/sites/y", "/healthz/extra"} {
		assertEnvelope(t, doGet(t, webuiEngine(t, true), http.MethodGet, path))
	}
}

func TestWebUIKeepsEnvelopeForNonGET(t *testing.T) {
	assertEnvelope(t, doGet(t, webuiEngine(t, true), http.MethodPost, "/some/client/route"))
	assertEnvelope(t, doGet(t, webuiEngine(t, true), http.MethodDelete, "/library"))
}

func TestNilWebUIKeepsEnvelopeEverywhere(t *testing.T) {
	assertEnvelope(t, doGet(t, webuiEngine(t, false), http.MethodGet, "/"))
	assertEnvelope(t, doGet(t, webuiEngine(t, false), http.MethodGet, "/library"))
}
