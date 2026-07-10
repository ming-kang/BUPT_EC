package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"reflect"

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

func isNilClassroomService(classroomService classroomDataService) bool {
	if classroomService == nil {
		return true
	}
	value := reflect.ValueOf(classroomService)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func NewHTTPServer(classroomService classroomDataService, hasJWCredentials func() bool) (*HTTPServer, error) {
	if isNilClassroomService(classroomService) {
		return nil, errors.New("classroom service is required")
	}
	if hasJWCredentials == nil {
		hasJWCredentials = func() bool { return false }
	}

	return &HTTPServer{
		classroomService: classroomService,
		hasJWCredentials: hasJWCredentials,
	}, nil
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
