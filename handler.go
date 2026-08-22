package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"BUPT_EC/logs"
	"BUPT_EC/service"
	"BUPT_EC/service/model"
)

type classroomDataService interface {
	GetTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error)
	GetRuntimeStatus() service.RuntimeStatus
	HasUsableTodayCache() bool
}

type HTTPServer struct {
	classroomService classroomDataService
	// hasJWCredentials is the startup credential predicate result. Runtime
	// config is immutable after Init, so a snapshot bool is equivalent.
	hasJWCredentials bool
	// readyzDiagnostics gates the full runtime diagnostics block on /readyz
	// (opt-in via READYZ_DIAGNOSTICS; public responses stay minimal).
	readyzDiagnostics bool
	metricsHandler    http.Handler
}

func NewHTTPServer(classroomService classroomDataService, hasJWCredentials bool, metricsHandler http.Handler) (*HTTPServer, error) {
	if classroomService == nil {
		return nil, errors.New("classroom service is required")
	}

	return &HTTPServer{
		classroomService: classroomService,
		hasJWCredentials: hasJWCredentials,
		metricsHandler:   metricsHandler,
	}, nil
}

// SetReadyzDiagnostics enables the full /readyz diagnostics block. Called
// once during Init from the loaded runtime config.
func (server *HTTPServer) SetReadyzDiagnostics(enabled bool) {
	server.readyzDiagnostics = enabled
}

func (server *HTTPServer) GetData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slog.InfoContext(ctx, "GetData")

	todayData, err := server.classroomService.GetTodayClassrooms(ctx)
	if err != nil {
		// Cold-start / in-flight-refresh failures are transient by design;
		// tell clients when to come back. Other 503 causes (config, auth)
		// stay header-free so retry hints never mask real misconfiguration.
		if errors.Is(err, service.ErrNoTodayCache) || errors.Is(err, service.ErrRefreshWaitTimeout) {
			w.Header().Set("Retry-After", "5")
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"code":    http.StatusServiceUnavailable,
			"msg":     service.SafeErrorMessage(err),
			"log_id":  logs.GetLogIDFromContext(ctx),
			"version": version,
			"data":    nil,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"log_id":  logs.GetLogIDFromContext(ctx),
		"version": version,
		"data":    todayData,
	})
}

func (server *HTTPServer) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (server *HTTPServer) Readyz(w http.ResponseWriter, _ *http.Request) {
	configured := server.hasJWCredentials
	ready := configured && server.classroomService.HasUsableTodayCache()
	code := http.StatusOK
	if !ready {
		code = http.StatusServiceUnavailable
	}

	// Default surface is minimal (status + version). Full runtime diagnostics
	// are opt-in via READYZ_DIAGNOSTICS so public deployments do not expose
	// login/refresh/cache internals.
	if !server.readyzDiagnostics {
		writeJSON(w, code, map[string]any{
			"status":  http.StatusText(code),
			"version": version,
		})
		return
	}

	writeJSON(w, code, map[string]any{
		"status":                    http.StatusText(code),
		"jw_credentials_configured": configured,
		"runtime":                   server.classroomService.GetRuntimeStatus(),
		"version":                   version,
	})
}

func (server *HTTPServer) Metrics(w http.ResponseWriter, r *http.Request) {
	if server.metricsHandler == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	server.metricsHandler.ServeHTTP(w, r)
}
