package main

import (
	"context"
	"log/slog"
	"net/http"

	"BUPT_EC/logs"
	"BUPT_EC/service"
	"BUPT_EC/service/model"

	"github.com/gin-gonic/gin"
)

type classroomDataService interface {
	GetTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error)
	GetRuntimeStatus() service.RuntimeStatus
	HasUsableTodayCache() bool
}

type HTTPServer struct {
	classroomService classroomDataService
	hasJWCredentials func() bool
}

func NewHTTPServer(classroomService classroomDataService, hasJWCredentials func() bool) *HTTPServer {
	if hasJWCredentials == nil {
		hasJWCredentials = func() bool { return false }
	}

	return &HTTPServer{
		classroomService: classroomService,
		hasJWCredentials: hasJWCredentials,
	}
}

func (server *HTTPServer) GetData(c *gin.Context) {
	ctx := logs.GetContextFromGinContext(c)
	slog.InfoContext(ctx, "GetData")

	todayData, err := server.classroomService.GetTodayClassrooms(ctx)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":   http.StatusServiceUnavailable,
			"msg":    service.SafeErrorMessage(err),
			"log_id": logs.GetLogIDFromContext(ctx),
			"data":   nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": todayData,
	})
}

func (server *HTTPServer) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (server *HTTPServer) Readyz(c *gin.Context) {
	status := server.classroomService.GetRuntimeStatus()
	configured := server.hasJWCredentials()
	ready := configured && server.classroomService.HasUsableTodayCache()
	code := http.StatusOK
	if !ready {
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{
		"status":                    http.StatusText(code),
		"jw_credentials_configured": configured,
		"runtime":                   status,
	})
}
