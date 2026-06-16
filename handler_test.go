package main

import (
	"BUPT_EC/cache"
	"BUPT_EC/config"
	"BUPT_EC/service"
	"BUPT_EC/service/model"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	config.InitConfig()
	cache.InitCache()
	gin.SetMode(gin.TestMode)
}

func TestReadyzRequiresUsableCache(t *testing.T) {
	service.ResetRuntimeStateForTest()
	t.Setenv(service.LoginTokenKey, "test-token")

	router := gin.New()
	router.GET("/readyz", Readyz)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz without cache status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	now := time.Now()
	cache.SetCache(service.TodayCacheKey, &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now,
		ExpiresAt:  now.Add(time.Minute),
		StaleUntil: now.Add(time.Hour),
	}, time.Hour)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("readyz with cache status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGzipMiddlewareCompressesAPIAndSkipsHealthz(t *testing.T) {
	router := gin.New()
	router.Use(gzipMiddleware())
	router.GET("/api/test", func(c *gin.Context) {
		c.String(http.StatusOK, strings.Repeat("x", 128))
	})
	router.GET("/healthz", Healthz)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	router.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", w.Header().Get("Content-Encoding"))
	}
	gz, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	body, err := io.ReadAll(gz)
	_ = gz.Close()
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if string(body) != strings.Repeat("x", 128) {
		t.Fatalf("unexpected decompressed body %q", string(body))
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	router.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") != "" {
		t.Fatalf("healthz Content-Encoding = %q, want empty", w.Header().Get("Content-Encoding"))
	}
}
