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
	metricsHandler   http.Handler
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

func (server *HTTPServer) GetData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slog.InfoContext(ctx, "GetData")

	todayData, err := server.classroomService.GetTodayClassrooms(ctx)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"code":   http.StatusServiceUnavailable,
			"msg":    service.SafeErrorMessage(err),
			"log_id": logs.GetLogIDFromContext(ctx),
			"data":   nil,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0,
		"data": todayData,
	})
}

func (server *HTTPServer) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (server *HTTPServer) Readyz(w http.ResponseWriter, _ *http.Request) {
	status := server.classroomService.GetRuntimeStatus()
	configured := server.hasJWCredentials
	ready := configured && server.classroomService.HasUsableTodayCache()
	code := http.StatusOK
	if !ready {
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, code, map[string]any{
		"status":                    http.StatusText(code),
		"jw_credentials_configured": configured,
		"runtime":                   status,
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
