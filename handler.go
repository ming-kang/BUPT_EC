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

// --- Typed response envelopes ---
// Using concrete structs instead of map[string]any eliminates per-request map
// and interface-box allocations and enables encoding/json's cached struct
// encoder (reflection done once globally, not per-call).

type getDataSuccessResponse struct {
	Code    int                    `json:"code"`
	LogID   string                 `json:"log_id"`
	Version string                 `json:"version"`
	Data    *model.TodayClassrooms `json:"data"`
}

type getDataErrorResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	LogID   string `json:"log_id"`
	Version string `json:"version"`
	Data    any    `json:"data"` // always nil → JSON null
}

type readyzMinimalResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type readyzDiagnosticsResponse struct {
	Status                  string                `json:"status"`
	JWCredentialsConfigured bool                  `json:"jw_credentials_configured"`
	Runtime                 service.RuntimeStatus `json:"runtime"`
	Version                 string                `json:"version"`
}

type notFoundResponse struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	LogID string `json:"log_id"`
}

// healthzBody is pre-computed once; /healthz achieves zero allocations per
// request by writing this constant slice directly.
var healthzBody = []byte(`{"status":"ok"}`)

type classroomDataService interface {
	GetTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error)
	GetRuntimeStatus() service.RuntimeStatus
	HasUsableTodayCache() bool
	// GetCachedDataJSON returns pre-serialized fresh TodayClassrooms JSON.
	// Returns nil, false when no fresh cache is available.
	GetCachedDataJSON() ([]byte, bool)
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

	// Fast path: if the pre-serialized fresh JSON is available, write the
	// envelope header + cached data bytes directly, skipping json.Marshal
	// of the full campus/room tree. This is the overwhelmingly common case
	// (data refreshes every 5 minutes; most requests hit the fresh cache).
	if dataJSON, ok := server.classroomService.GetCachedDataJSON(); ok {
		writePreserializedGetData(w, http.StatusOK, logs.GetLogIDFromContext(ctx), version, dataJSON)
		return
	}

	// Slow path: stale, partial, cold-start, or error responses.
	todayData, err := server.classroomService.GetTodayClassrooms(ctx)
	if err != nil {
		// Cold-start / in-flight-refresh failures are transient by design;
		// tell clients when to come back. Other 503 causes (config, auth)
		// stay header-free so retry hints never mask real misconfiguration.
		if errors.Is(err, service.ErrNoTodayCache) || errors.Is(err, service.ErrRefreshWaitTimeout) {
			w.Header().Set("Retry-After", "5")
		}
		writeJSON(w, http.StatusServiceUnavailable, &getDataErrorResponse{
			Code:    http.StatusServiceUnavailable,
			Msg:     service.SafeErrorMessage(err),
			LogID:   logs.GetLogIDFromContext(ctx),
			Version: version,
			Data:    nil,
		})
		return
	}

	writeJSON(w, http.StatusOK, &getDataSuccessResponse{
		Code:    0,
		LogID:   logs.GetLogIDFromContext(ctx),
		Version: version,
		Data:    todayData,
	})
}

func (server *HTTPServer) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(healthzBody)
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
		writeJSON(w, code, &readyzMinimalResponse{
			Status:  http.StatusText(code),
			Version: version,
		})
		return
	}

	writeJSON(w, code, &readyzDiagnosticsResponse{
		Status:                  http.StatusText(code),
		JWCredentialsConfigured: configured,
		Runtime:                 server.classroomService.GetRuntimeStatus(),
		Version:                 version,
	})
}

func (server *HTTPServer) Metrics(w http.ResponseWriter, r *http.Request) {
	if server.metricsHandler == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	server.metricsHandler.ServeHTTP(w, r)
}
