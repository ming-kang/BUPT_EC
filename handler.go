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

// getDataResponse is the /api/get_data envelope: code 0 + data on success,
// code 503 + msg + null data on failure. Both carry log_id and version.
type getDataResponse struct {
	Code    int                    `json:"code"`
	Msg     string                 `json:"msg,omitempty"`
	LogID   string                 `json:"log_id"`
	Version string                 `json:"version"`
	Data    *model.TodayClassrooms `json:"data"`
}

type statusResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

type notFoundResponse struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	LogID string `json:"log_id"`
}

type classroomService interface {
	GetTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error)
	Ready() bool
}

type HTTPServer struct {
	classrooms classroomService
}

func NewHTTPServer(classrooms classroomService) *HTTPServer {
	return &HTTPServer{classrooms: classrooms}
}

func isWarmingError(err error) bool {
	return errors.Is(err, service.ErrNoTodayCache) || errors.Is(err, service.ErrRefreshWaitTimeout)
}

func (server *HTTPServer) GetData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slog.InfoContext(ctx, "GetData")
	logID := logs.GetLogIDFromContext(ctx)

	data, err := server.classrooms.GetTodayClassrooms(ctx)
	if err != nil {
		if isWarmingError(err) {
			w.Header().Set("Retry-After", "5")
		}
		writeJSON(w, http.StatusServiceUnavailable, &getDataResponse{
			Code:    http.StatusServiceUnavailable,
			Msg:     service.SafeErrorMessage(err),
			LogID:   logID,
			Version: version,
		})
		return
	}
	writeJSON(w, http.StatusOK, &getDataResponse{LogID: logID, Version: version, Data: data})
}

func (server *HTTPServer) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, &statusResponse{Status: "ok"})
}

func (server *HTTPServer) Readyz(w http.ResponseWriter, _ *http.Request) {
	code := http.StatusOK
	if !server.classrooms.Ready() {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, &statusResponse{Status: http.StatusText(code), Version: version})
}
