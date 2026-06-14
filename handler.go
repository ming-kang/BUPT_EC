package main

import (
	"BUPT_EC/config"
	"BUPT_EC/logs"
	"BUPT_EC/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetData(c *gin.Context) {
	ctx := logs.GetContextFromGinContext(c)
	logs.CtxInfo(ctx, "GetData")

	todayData, err := service.GetTodayClassrooms(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
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
	code := http.StatusOK
	if !configured {
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{
		"status":                    http.StatusText(code),
		"jw_credentials_configured": configured,
		"runtime":                   status,
	})
}
