package main

import (
	"BUPT_EC/service"
	"BUPT_EC/service/model"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

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

func newTestHTTPServer(classroomService *fakeClassroomService, hasJWCredentials func() bool) *HTTPServer {
	if classroomService == nil {
		classroomService = &fakeClassroomService{}
	}
	if hasJWCredentials == nil {
		hasJWCredentials = func() bool { return true }
	}
	return NewHTTPServer(classroomService, hasJWCredentials)
}

func TestReadyzRequiresConfiguredCredentialsAndUsableCache(t *testing.T) {
	classroomService := &fakeClassroomService{usableTodayCache: true}
	credentialsConfigured := false
	httpServer := newTestHTTPServer(classroomService, func() bool {
		return credentialsConfigured
	})

	router := gin.New()
	router.GET("/readyz", httpServer.Readyz)

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz without credentials status = %d, want %d", responseRecorder.Code, http.StatusServiceUnavailable)
	}

	credentialsConfigured = true
	classroomService.usableTodayCache = false
	responseRecorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz without cache status = %d, want %d", responseRecorder.Code, http.StatusServiceUnavailable)
	}

	classroomService.usableTodayCache = true
	responseRecorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("readyz with credentials and cache status = %d, want %d", responseRecorder.Code, http.StatusOK)
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
	}, nil)

	router := gin.New()
	httpServer.RegisterRoutes(router)

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/get_data", nil)
	router.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("GetData status = %d, want %d", responseRecorder.Code, http.StatusOK)
	}

	var envelope struct {
		Code int                    `json:"code"`
		Data *model.TodayClassrooms `json:"data"`
	}
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode GetData response: %v", err)
	}
	if envelope.Code != 0 {
		t.Fatalf("GetData code = %d, want 0", envelope.Code)
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
	httpServer := newTestHTTPServer(&fakeClassroomService{todayError: upstreamError}, nil)

	router := gin.New()
	httpServer.RegisterRoutes(router)

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/get_data", nil)
	router.ServeHTTP(responseRecorder, request)
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
	logIDHeader := responseRecorder.Header().Get("LogID")
	if logIDHeader == "" {
		t.Fatal("GetData error response should include a non-empty LogID header")
	}
	if envelope.LogID != logIDHeader {
		t.Fatalf("GetData error log_id = %q, want header LogID %q", envelope.LogID, logIDHeader)
	}
	if strings.Contains(responseRecorder.Body.String(), upstreamError.Error()) {
		t.Fatalf("GetData error response leaked raw error detail: %s", responseRecorder.Body.String())
	}
}

func TestNoRouteServesSPAFallback(t *testing.T) {
	router := gin.New()
	newTestHTTPServer(nil, nil).RegisterRoutes(router)

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/some/client/route", nil)
	router.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("SPA fallback status = %d, want %d", responseRecorder.Code, http.StatusOK)
	}
	if contentType := responseRecorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("SPA fallback Content-Type = %q, want text/html", contentType)
	}

	responseRecorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	router.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("unknown api route status = %d, want %d", responseRecorder.Code, http.StatusNotFound)
	}
	if contentType := responseRecorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("unknown api route Content-Type = %q, want application/json", contentType)
	}

	responseRecorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api", nil)
	router.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("GET /api status = %d, want %d", responseRecorder.Code, http.StatusNotFound)
	}
	if contentType := responseRecorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("GET /api Content-Type = %q, want application/json", contentType)
	}
}

func TestGzipMiddlewareCompressesAPIAndSkipsHealthz(t *testing.T) {
	httpServer := newTestHTTPServer(nil, nil)
	router := gin.New()
	router.Use(gzipMiddleware())
	router.GET("/api/test", func(c *gin.Context) {
		c.String(http.StatusOK, strings.Repeat("x", 128))
	})
	router.GET("/healthz", httpServer.Healthz)

	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	router.ServeHTTP(responseRecorder, request)
	if responseRecorder.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", responseRecorder.Header().Get("Content-Encoding"))
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
	if string(body) != strings.Repeat("x", 128) {
		t.Fatalf("unexpected decompressed body %q", string(body))
	}

	responseRecorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	router.ServeHTTP(responseRecorder, request)
	if responseRecorder.Header().Get("Content-Encoding") != "" {
		t.Fatalf("healthz Content-Encoding = %q, want empty", responseRecorder.Header().Get("Content-Encoding"))
	}
}
