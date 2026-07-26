package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"BUPT_EC/web"
)

const immutableCacheControl = "public, max-age=31536000, immutable"

func serveStatic(t *testing.T, handler http.Handler, method, target string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, nil)
	for key, value := range header {
		request.Header.Set(key, value)
	}
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestImmutableCacheSetsHeader(t *testing.T) {
	called := false
	handler := immutableCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if !called {
		t.Fatal("immutableCache must delegate to the wrapped handler")
	}
	if got := recorder.Header().Get("Cache-Control"); got != immutableCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, immutableCacheControl)
	}
}

func TestAssetsMissingFileReturnsNotFound(t *testing.T) {
	handler := newTestHTTPServer(nil, true).Routes()

	// An /assets/ miss must be a true 404 from the file server, never the SPA
	// fallback (previously a missing asset name incorrectly returned
	// index.html with 200). Note: the stdlib file server error path strips
	// Cache-Control on 404s (net/http serveError), so no header assertion.
	recorder := serveStatic(t, handler, http.MethodGet, "/assets/missing.js", nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("/assets/missing.js status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if contentType := recorder.Header().Get("Content-Type"); strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("/assets/missing.js Content-Type = %q, must not be the SPA fallback HTML", contentType)
	}
}

func TestAssetsDirectoryListingIsRejected(t *testing.T) {
	handler := newTestHTTPServer(nil, true).Routes()

	// Directory paths under /assets/ must 404 instead of producing a file
	// server listing: listings have no content hash, so neither the listing
	// body nor the immutable Cache-Control header may ever be served.
	recorder := serveStatic(t, handler, http.MethodGet, "/assets/", nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("/assets/ status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("Cache-Control"); strings.Contains(got, "immutable") {
		t.Fatalf("/assets/ Cache-Control = %q, must not be immutable", got)
	}
}

func TestEmbeddedAssetsCarryImmutableCacheControl(t *testing.T) {
	distFS, embedded := web.Dist()
	if !embedded {
		t.Skip("real hashed assets exist only in -tags embed_assets builds")
	}
	names, err := fs.Glob(distFS, "assets/*")
	if err != nil || len(names) == 0 {
		t.Fatalf("embedded build has no assets/ files (err = %v)", err)
	}

	handler := newTestHTTPServer(nil, true).Routes()
	recorder := serveStatic(t, handler, http.MethodGet, "/"+names[0], nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("/%s status = %d, want %d", names[0], recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Cache-Control"); got != immutableCacheControl {
		t.Fatalf("/%s Cache-Control = %q, want %q", names[0], got, immutableCacheControl)
	}
	if recorder.Body.Len() == 0 {
		t.Fatalf("/%s body is empty", names[0])
	}
}

func TestIndexAndSPAFallbackUseWeakETagAndNoCache(t *testing.T) {
	handler := newTestHTTPServer(nil, true).Routes()

	var etag string
	for _, path := range []string{"/", "/some/deep/link", "/index.html"} {
		recorder := serveStatic(t, handler, http.MethodGet, path, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, recorder.Code, http.StatusOK)
		}
		if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
			t.Fatalf("%s Content-Type = %q, want text/html", path, contentType)
		}
		if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-cache" {
			t.Fatalf("%s Cache-Control = %q, want no-cache", path, cacheControl)
		}
		got := recorder.Header().Get("ETag")
		if !strings.HasPrefix(got, `W/"`) || !strings.HasSuffix(got, `"`) {
			t.Fatalf(`%s ETag = %q, want weak W/"..." validator`, path, got)
		}
		if etag == "" {
			etag = got
		} else if got != etag {
			t.Fatalf("%s ETag = %q, want the consistent index ETag %q", path, got, etag)
		}
		if recorder.Body.Len() == 0 {
			t.Fatalf("%s body is empty", path)
		}
	}
}

func TestIndexIfNoneMatchReturnsNotModified(t *testing.T) {
	handler := newTestHTTPServer(nil, true).Routes()

	first := serveStatic(t, handler, http.MethodGet, "/", nil)
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("GET / missing ETag")
	}

	second := serveStatic(t, handler, http.MethodGet, "/", map[string]string{"If-None-Match": etag})
	if second.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match status = %d, want %d", second.Code, http.StatusNotModified)
	}
	if second.Body.Len() != 0 {
		t.Fatalf("304 body should be empty, got %q", second.Body.String())
	}
}

func TestUnknownAPIMethodAndPathReturnJSONNotFound(t *testing.T) {
	handler := newTestHTTPServer(nil, true).Routes()

	// The catch-all "/" pattern is method-independent, so a POST to an
	// unknown API path must produce the JSON 404 envelope, never a 405.
	recorder := serveStatic(t, handler, http.MethodPost, "/api/unknown", nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("POST /api/unknown status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("POST /api/unknown Content-Type = %q, want application/json", contentType)
	}
	var envelope struct {
		Code  int    `json:"code"`
		Msg   string `json:"msg"`
		LogID string `json:"log_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode POST /api/unknown: %v", err)
	}
	if envelope.Code != http.StatusNotFound || envelope.Msg != "not found" {
		t.Fatalf("POST /api/unknown envelope = %#v", envelope)
	}
	logIDHeader := recorder.Header().Get("X-Log-Id")
	if logIDHeader == "" || envelope.LogID != logIDHeader {
		t.Fatalf("POST /api/unknown log_id = %q header = %q, want matching non-empty values", envelope.LogID, logIDHeader)
	}
}
