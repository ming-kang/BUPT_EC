package main

import (
	"BUPT_EC/config"
	"BUPT_EC/logs"
	"BUPT_EC/service"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetData(c *gin.Context) {
	ctx := logs.GetContextFromGinContext(c)
	slog.InfoContext(ctx, "GetData")

	todayData, err := service.GetTodayClassrooms(ctx)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code": http.StatusServiceUnavailable,
			"msg":  service.SafeErrorMessage(err),
			"data": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": todayData,
	})
}

func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func Readyz(c *gin.Context) {
	status := service.GetRuntimeStatus()
	configured := config.HasJWCredentials()
	ready := configured && service.HasUsableTodayCache()
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
