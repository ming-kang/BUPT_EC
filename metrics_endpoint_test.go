package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"BUPT_EC/service"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

func newTestMetricsHandler(t *testing.T, observe func(*service.PrometheusMetrics)) http.Handler {
	t.Helper()
	metrics, err := service.NewPrometheusMetrics()
	if err != nil {
		t.Fatalf("NewPrometheusMetrics() error = %v", err)
	}
	if observe != nil {
		observe(metrics)
	}
	// Production main.go uses the same HandlerOpts; tests must keep them aligned.
	return promhttp.HandlerFor(metrics.Registry(), promhttp.HandlerOpts{
		DisableCompression: true,
	})
}

func newMetricsTestRouter(t *testing.T, metricsHandler http.Handler) *gin.Engine {
	t.Helper()
	httpServer, err := NewHTTPServer(&fakeClassroomService{}, func() bool { return true }, metricsHandler)
	if err != nil {
		t.Fatalf("NewHTTPServer() error = %v", err)
	}
	router := gin.New()
	router.Use(gzipMiddleware())
	router.GET("/metrics", httpServer.Metrics)
	router.GET("/healthz", httpServer.Healthz)
	router.GET("/readyz", httpServer.Readyz)
	return router
}

func TestMetricsEndpointIdentityAndGzipOnce(t *testing.T) {
	handler := newTestMetricsHandler(t, func(m *service.PrometheusMetrics) {
		m.ObserveLogin("success", "login", 100*time.Millisecond)
		m.ObserveRefresh("full", time.Second)
		m.ObserveCacheServe("fresh")
	})
	router := newMetricsTestRouter(t, handler)

	// Identity (no Accept-Encoding): uncompressed Prometheus text.
	identityBody := serveMetrics(t, router, "")
	if identityBody.Header().Get("Content-Encoding") != "" {
		t.Fatalf("identity Content-Encoding = %q, want empty", identityBody.Header().Get("Content-Encoding"))
	}
	assertPrometheusText(t, identityBody.Body.Bytes())
	assertContainsMetricFamily(t, identityBody.Body.Bytes(), "bupt_ec_login_total")

	// Standard gzip: one Content-Encoding, one decompress yields text.
	gzipResp := serveMetrics(t, router, "gzip")
	if gzipResp.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("gzip Content-Encoding = %q, want gzip", gzipResp.Header().Get("Content-Encoding"))
	}
	if !strings.Contains(gzipResp.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("Vary = %q, want Accept-Encoding", gzipResp.Header().Get("Vary"))
	}
	// Wire body must itself be gzip (magic 1f 8b).
	raw := gzipResp.Body.Bytes()
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		t.Fatalf("gzip body missing gzip magic: %x", raw[:min(4, len(raw))])
	}
	decompressed := gunzipOnce(t, raw)
	// After one decompress, body must be Prometheus text — not another gzip layer.
	if len(decompressed) >= 2 && decompressed[0] == 0x1f && decompressed[1] == 0x8b {
		t.Fatal("metrics body is still gzip after one decompress (double compression)")
	}
	assertPrometheusText(t, decompressed)
	assertContainsMetricFamily(t, decompressed, "bupt_ec_login_total")
	assertContainsMetricFamily(t, decompressed, "bupt_ec_refresh_total")

	// gzip;q=0 and explicit identity preference stay uncompressed.
	for _, accept := range []string{"gzip;q=0", "identity, gzip;q=0"} {
		resp := serveMetrics(t, router, accept)
		if resp.Header().Get("Content-Encoding") != "" {
			t.Fatalf("%q Content-Encoding = %q, want empty", accept, resp.Header().Get("Content-Encoding"))
		}
		assertPrometheusText(t, resp.Body.Bytes())
	}

	// Wildcard Accept-Encoding may gzip; still only one layer.
	starResp := serveMetrics(t, router, "*")
	if starResp.Header().Get("Content-Encoding") == "gzip" {
		body := gunzipOnce(t, starResp.Body.Bytes())
		if len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b {
			t.Fatal("wildcard gzip still double-compressed")
		}
		assertPrometheusText(t, body)
	} else {
		assertPrometheusText(t, starResp.Body.Bytes())
	}
}

func TestMetricsEndpointHealthzAndReadyzStayUncompressed(t *testing.T) {
	handler := newTestMetricsHandler(t, nil)
	router := newMetricsTestRouter(t, handler)

	for _, path := range []string{"/healthz", "/readyz"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Accept-Encoding", "gzip")
		router.ServeHTTP(recorder, request)
		if recorder.Header().Get("Content-Encoding") != "" {
			t.Fatalf("%s Content-Encoding = %q, want empty", path, recorder.Header().Get("Content-Encoding"))
		}
	}
}

func TestMetricsEndpointMissingHandlerIsNotFound(t *testing.T) {
	router := newMetricsTestRouter(t, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func serveMetrics(t *testing.T, router http.Handler, acceptEncoding string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	if acceptEncoding != "" {
		request.Header.Set("Accept-Encoding", acceptEncoding)
	}
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", recorder.Code, recorder.Body.String())
	}
	return recorder
}

func gunzipOnce(t *testing.T, raw []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}
	return body
}

func assertPrometheusText(t *testing.T, body []byte) {
	t.Helper()
	text := string(body)
	if !strings.Contains(text, "# HELP") && !strings.Contains(text, "# TYPE") {
		// Empty registry still produces valid exposition; allow parse-only path.
		if len(bytes.TrimSpace(body)) == 0 {
			t.Fatal("empty metrics body")
		}
	}
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("parse prometheus text: %v\nbody prefix: %q", err, truncateForTest(text, 200))
	}
	if len(families) == 0 {
		t.Fatal("parsed zero metric families")
	}
	// Ensure dto types are linked so accidental format regressions fail loudly.
	for _, family := range families {
		if family.GetType() == dto.MetricType_UNTYPED && family.GetName() == "" {
			t.Fatal("invalid empty untyped family")
		}
	}
}

func assertContainsMetricFamily(t *testing.T, body []byte, name string) {
	t.Helper()
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("parse prometheus text: %v", err)
	}
	if _, ok := families[name]; !ok {
		t.Fatalf("missing metric family %q in exposition", name)
	}
}

func truncateForTest(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
