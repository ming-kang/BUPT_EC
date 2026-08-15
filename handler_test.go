package main

import (
	"BUPT_EC/service"
	"BUPT_EC/service/model"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

type fakeClassroomService struct {
	todayClassrooms  *model.TodayClassrooms
	todayError       error
	runtimeStatus    service.RuntimeStatus
	usableTodayCache bool
}

func (classroomService *fakeClassroomService) GetTodayClassrooms(_ context.Context) (*model.TodayClassrooms, error) {
	return classroomService.todayClassrooms, classroomService.todayError
}

func (classroomService *fakeClassroomService) GetRuntimeStatus() service.RuntimeStatus {
	return classroomService.runtimeStatus
}

func (classroomService *fakeClassroomService) HasUsableTodayCache() bool {
	return classroomService.usableTodayCache
}

func newTestHTTPServer(classroomService *fakeClassroomService, hasJWCredentials bool) *HTTPServer {
	if classroomService == nil {
		classroomService = &fakeClassroomService{}
	}
	httpServer, err := NewHTTPServer(classroomService, hasJWCredentials, nil)
	if err != nil {
		panic(err)
	}
	return httpServer
}

func TestNewHTTPServerRejectsNilService(t *testing.T) {
	if _, err := NewHTTPServer(nil, false, nil); err == nil {
		t.Fatal("NewHTTPServer(nil) expected error")
	}
	if _, err := NewHTTPServer(&fakeClassroomService{}, false, nil); err != nil {
		t.Fatalf("NewHTTPServer(valid) error = %v", err)
	}
}

func TestReadyzRequiresConfiguredCredentialsAndUsableCache(t *testing.T) {
	serveReadyz := func(httpServer *HTTPServer) *httptest.ResponseRecorder {
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		http.HandlerFunc(httpServer.Readyz).ServeHTTP(responseRecorder, request)
		return responseRecorder
	}

	// Credentials missing: not ready even with a usable cache.
	withoutCredentials := newTestHTTPServer(&fakeClassroomService{usableTodayCache: true}, false)
	if code := serveReadyz(withoutCredentials).Code; code != http.StatusServiceUnavailable {
		t.Fatalf("readyz without credentials status = %d, want %d", code, http.StatusServiceUnavailable)
	}

	// Credentials configured but no usable cache: still not ready.
	classroomService := &fakeClassroomService{usableTodayCache: false}
	withCredentials := newTestHTTPServer(classroomService, true)
	if code := serveReadyz(withCredentials).Code; code != http.StatusServiceUnavailable {
		t.Fatalf("readyz without cache status = %d, want %d", code, http.StatusServiceUnavailable)
	}

	// Cache becomes usable: ready.
	classroomService.usableTodayCache = true
	if code := serveReadyz(withCredentials).Code; code != http.StatusOK {
		t.Fatalf("readyz with credentials and cache status = %d, want %d", code, http.StatusOK)
	}
}

func TestReadyzDefaultsToMinimalSurfaceWithoutDiagnostics(t *testing.T) {
	serveReadyz := func(httpServer *HTTPServer) *httptest.ResponseRecorder {
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		http.HandlerFunc(httpServer.Readyz).ServeHTTP(responseRecorder, request)
		return responseRecorder
	}

	httpServer := newTestHTTPServer(&fakeClassroomService{usableTodayCache: true}, true)
	responseRecorder := serveReadyz(httpServer)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("readyz status = %d, want %d", responseRecorder.Code, http.StatusOK)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode readyz response: %v", err)
	}
	for _, key := range []string{"status", "version"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("readyz minimal surface missing %q: %s", key, responseRecorder.Body.String())
		}
	}
	for _, key := range []string{"runtime", "jw_credentials_configured"} {
		if _, ok := body[key]; ok {
			t.Fatalf("readyz minimal surface must not expose %q: %s", key, responseRecorder.Body.String())
		}
	}

	// Not-ready keeps 503 on the minimal surface.
	notReady := newTestHTTPServer(&fakeClassroomService{usableTodayCache: false}, true)
	if code := serveReadyz(notReady).Code; code != http.StatusServiceUnavailable {
		t.Fatalf("readyz not-ready status = %d, want %d", code, http.StatusServiceUnavailable)
	}
}

func TestReadyzReportsPartialCacheDiagnostics(t *testing.T) {
	httpServer := newTestHTTPServer(&fakeClassroomService{
		usableTodayCache: true,
		runtimeStatus: service.RuntimeStatus{
			CacheAvailable:     true,
			CacheFresh:         true,
			CachePartial:       true,
			PartialCampuses:    []string{"04"},
			LastRefreshWarning: "部分校区数据刷新失败，已展示可用数据",
		},
	}, true)
	httpServer.SetReadyzDiagnostics(true)

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	http.HandlerFunc(httpServer.Readyz).ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("partial cache readyz status = %d, want %d", responseRecorder.Code, http.StatusOK)
	}

	var body struct {
		Runtime service.RuntimeStatus `json:"runtime"`
		Version string                `json:"version"`
	}
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode readyz partial response: %v", err)
	}
	if !body.Runtime.CachePartial || !reflect.DeepEqual(body.Runtime.PartialCampuses, []string{"04"}) {
		t.Fatalf("readyz partial runtime = %#v", body.Runtime)
	}
	if body.Runtime.LastRefreshWarning == "" {
		t.Fatalf("readyz missing partial warning: %#v", body.Runtime)
	}
	if body.Version != "dev" {
		t.Fatalf("readyz version = %q, want default %q", body.Version, "dev")
	}
}

func TestGetDataReturnsSuccessEnvelopeFromInjectedService(t *testing.T) {
	now := time.Now()
	httpServer := newTestHTTPServer(&fakeClassroomService{
		todayClassrooms: &model.TodayClassrooms{
			Date:       now.Format("2006-01-02"),
			UpdatedAt:  now,
			ExpiresAt:  now.Add(time.Minute),
			StaleUntil: now.Add(time.Hour),
			Campuses: []model.CampusInfo{
				{ID: "01", Name: "西土城"},
			},
		},
	}, true)

	handler := httpServer.Routes()

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/get_data", nil)
	handler.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("GetData status = %d, want %d", responseRecorder.Code, http.StatusOK)
	}

	var envelope struct {
		Code  int                    `json:"code"`
		LogID string                 `json:"log_id"`
		Data  *model.TodayClassrooms `json:"data"`
	}
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode GetData response: %v", err)
	}
	if envelope.Code != 0 {
		t.Fatalf("GetData code = %d, want 0", envelope.Code)
	}
	if envelope.LogID == "" {
		t.Fatal("GetData success envelope should carry a non-empty log_id")
	}
	if envelope.Data == nil {
		t.Fatal("GetData data should not be nil")
	}
	if envelope.Data.Stale {
		t.Fatal("GetData fresh cache response should not be stale")
	}
	if len(envelope.Data.Campuses) != 1 || envelope.Data.Campuses[0].ID != "01" {
		t.Fatalf("GetData campuses = %#v, want campus 01", envelope.Data.Campuses)
	}
}

func TestGetDataReturnsSafeErrorEnvelopeWithLogID(t *testing.T) {
	upstreamError := errors.New("raw upstream token detail should not leak")
	httpServer := newTestHTTPServer(&fakeClassroomService{todayError: upstreamError}, true)

	handler := httpServer.Routes()

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/get_data", nil)
	handler.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("GetData error status = %d, want %d", responseRecorder.Code, http.StatusServiceUnavailable)
	}

	var envelope struct {
		Code  int                    `json:"code"`
		Msg   string                 `json:"msg"`
		LogID string                 `json:"log_id"`
		Data  *model.TodayClassrooms `json:"data"`
	}
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode GetData error response: %v", err)
	}
	if envelope.Code != http.StatusServiceUnavailable {
		t.Fatalf("GetData error code = %d, want %d", envelope.Code, http.StatusServiceUnavailable)
	}
	if envelope.Msg != service.SafeErrorMessage(upstreamError) {
		t.Fatalf("GetData error msg = %q, want safe message %q", envelope.Msg, service.SafeErrorMessage(upstreamError))
	}
	if envelope.Data != nil {
		t.Fatalf("GetData error data = %#v, want nil", envelope.Data)
	}
	logIDHeader := responseRecorder.Header().Get("X-Log-Id")
	if logIDHeader == "" {
		t.Fatal("GetData error response should include a non-empty X-Log-Id header")
	}
	if envelope.LogID != logIDHeader {
		t.Fatalf("GetData error log_id = %q, want header X-Log-Id %q", envelope.LogID, logIDHeader)
	}
	if strings.Contains(responseRecorder.Body.String(), upstreamError.Error()) {
		t.Fatalf("GetData error response leaked raw error detail: %s", responseRecorder.Body.String())
	}
}

func TestNoRouteServesSPAFallback(t *testing.T) {
	handler := newTestHTTPServer(nil, true).Routes()

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/some/client/route", nil)
	handler.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("SPA fallback status = %d, want %d", responseRecorder.Code, http.StatusOK)
	}
	if contentType := responseRecorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("SPA fallback Content-Type = %q, want text/html", contentType)
	}
	if logID := responseRecorder.Header().Get("X-Log-Id"); logID != "" {
		t.Fatalf("SPA fallback must not force X-Log-Id header, got %q", logID)
	}

	for _, path := range []string{"/api/nonexistent", "/api"} {
		responseRecorder = httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodGet, path, nil)
		handler.ServeHTTP(responseRecorder, request)
		if responseRecorder.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d", path, responseRecorder.Code, http.StatusNotFound)
		}
		if contentType := responseRecorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
			t.Fatalf("%s Content-Type = %q, want application/json", path, contentType)
		}
		var envelope struct {
			Code  int    `json:"code"`
			Msg   string `json:"msg"`
			LogID string `json:"log_id"`
		}
		if err := json.Unmarshal(responseRecorder.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("%s decode: %v", path, err)
		}
		logIDHeader := responseRecorder.Header().Get("X-Log-Id")
		if logIDHeader == "" {
			t.Fatalf("%s missing X-Log-Id header", path)
		}
		if envelope.LogID == "" || envelope.LogID != logIDHeader {
			t.Fatalf("%s log_id = %q header = %q, want matching non-empty values", path, envelope.LogID, logIDHeader)
		}
		if envelope.Code != http.StatusNotFound || envelope.Msg != "not found" {
			t.Fatalf("%s envelope = %#v", path, envelope)
		}
	}
}

func TestRecoveryConvertsPanicToCleanInternalError(t *testing.T) {
	const secretDetail = "secret panic detail must never reach the client"
	handler := recovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(secretDetail)
	}))

	responseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(responseRecorder, httptest.NewRequest(http.MethodGet, "/api/get_data", nil))
	if responseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("recovered panic status = %d, want %d", responseRecorder.Code, http.StatusInternalServerError)
	}
	if body := responseRecorder.Body.String(); strings.Contains(body, secretDetail) || body != "" {
		t.Fatalf("recovered panic body = %q, want empty (no panic detail leak)", body)
	}

	// A panic after the response started must not overwrite the status.
	handler = recovery(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		panic(secretDetail)
	}))
	responseRecorder = httptest.NewRecorder()
	handler.ServeHTTP(responseRecorder, httptest.NewRequest(http.MethodGet, "/api/get_data", nil))
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("post-write panic status = %d, want %d (header already sent)", responseRecorder.Code, http.StatusOK)
	}
	if strings.Contains(responseRecorder.Body.String(), secretDetail) {
		t.Fatalf("post-write panic leaked detail: %q", responseRecorder.Body.String())
	}
}

// largeTodayClassrooms returns a payload whose JSON encoding comfortably
// exceeds the gzhttp MinSize threshold (1024 bytes) so compression triggers.
func largeTodayClassrooms(now time.Time) *model.TodayClassrooms {
	campuses := make([]model.CampusInfo, 0, 32)
	for i := range 32 {
		campuses = append(campuses, model.CampusInfo{
			ID:   fmt.Sprintf("%02d", i),
			Name: fmt.Sprintf("测试校区-%02d", i),
		})
	}
	return &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now,
		ExpiresAt:  now.Add(time.Minute),
		StaleUntil: now.Add(time.Hour),
		Campuses:   campuses,
	}
}

func TestGzipCompressesAPIAndSkipsHealthz(t *testing.T) {
	httpServer := newTestHTTPServer(&fakeClassroomService{
		todayClassrooms:  largeTodayClassrooms(time.Now()),
		usableTodayCache: true,
	}, true)
	handler := httpServer.Routes()

	// Identity baseline: no Accept-Encoding means no compression.
	identityRecorder := httptest.NewRecorder()
	identityRequest := httptest.NewRequest(http.MethodGet, "/api/get_data", nil)
	handler.ServeHTTP(identityRecorder, identityRequest)
	if identityRecorder.Code != http.StatusOK {
		t.Fatalf("identity status = %d, want %d", identityRecorder.Code, http.StatusOK)
	}
	if encoding := identityRecorder.Header().Get("Content-Encoding"); encoding != "" {
		t.Fatalf("identity Content-Encoding = %q, want empty", encoding)
	}
	identityBody := identityRecorder.Body.String()
	if len(identityBody) < 1024 {
		t.Fatalf("fixture body = %d bytes, must be >= 1024 to exceed gzhttp MinSize", len(identityBody))
	}
	// Each request gets a fresh log_id; strip it before byte comparison.
	stripLogID := func(body string) string {
		return regexp.MustCompile(`"log_id":"[^"]*"`).ReplaceAllString(body, `"log_id":""`)
	}
	identityBody = stripLogID(identityBody)

	// Compressible API response with Accept-Encoding: gzip is compressed once.
	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/get_data", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	handler.ServeHTTP(responseRecorder, request)
	if responseRecorder.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", responseRecorder.Header().Get("Content-Encoding"))
	}
	if !strings.Contains(responseRecorder.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("Vary = %q, want Accept-Encoding", responseRecorder.Header().Get("Vary"))
	}
	gzipReader, err := gzip.NewReader(responseRecorder.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	body, err := io.ReadAll(gzipReader)
	_ = gzipReader.Close()
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if stripLogID(string(body)) != identityBody {
		t.Fatalf("decompressed body differs from identity body:\n%q\nvs\n%q", stripLogID(string(body)), identityBody)
	}

	// gzip;q=0 negotiates identity.
	responseRecorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/get_data", nil)
	request.Header.Set("Accept-Encoding", "gzip;q=0")
	handler.ServeHTTP(responseRecorder, request)
	if responseRecorder.Header().Get("Content-Encoding") != "" {
		t.Fatalf("gzip;q=0 Content-Encoding = %q, want empty", responseRecorder.Header().Get("Content-Encoding"))
	}
	if stripLogID(responseRecorder.Body.String()) != identityBody {
		t.Fatal("gzip;q=0 body should remain identity")
	}

	// /healthz bypasses the gzip wrapper entirely: no compression, no Vary.
	responseRecorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	handler.ServeHTTP(responseRecorder, request)
	if responseRecorder.Header().Get("Content-Encoding") != "" {
		t.Fatalf("healthz Content-Encoding = %q, want empty", responseRecorder.Header().Get("Content-Encoding"))
	}
	if vary := responseRecorder.Header().Get("Vary"); vary != "" {
		t.Fatalf("healthz Vary = %q, want empty (probe bypasses the wrapper)", vary)
	}
}

// TestGzipWrapperSkipsAlreadyCompressedAssetTypes locks the acceptance
// criterion that png/woff2 (and other binary types outside the allowlist) are
// never re-compressed, even for large bodies from clients that accept gzip.
// It uses the production newGzipWrapper so the allowlist cannot drift
// (gzhttp's *default* content-type filter would compress png and woff2).
func TestGzipWrapperSkipsAlreadyCompressedAssetTypes(t *testing.T) {
	payload := bytes.Repeat([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, 512) // 4KB > MinSize
	for _, contentType := range []string{"image/png", "font/woff2", "application/octet-stream"} {
		handler := newGzipWrapper()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", contentType)
			_, _ = w.Write(payload)
		}))

		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/assets/asset.bin", nil)
		request.Header.Set("Accept-Encoding", "gzip")
		handler.ServeHTTP(responseRecorder, request)
		if encoding := responseRecorder.Header().Get("Content-Encoding"); encoding != "" {
			t.Fatalf("%s Content-Encoding = %q, want empty (must not re-compress)", contentType, encoding)
		}
		if !bytes.Equal(responseRecorder.Body.Bytes(), payload) {
			t.Fatalf("%s body was altered by the wrapper", contentType)
		}
	}
}
