package main

import (
	"BUPT_EC/logs"
	"BUPT_EC/service"
	"github.com/gin-gonic/gin"
)

func GetData(c *gin.Context) {
	ctx := logs.GetContextFromGinContext(c)
	logs.CtxInfo(ctx, "GetData")
	service.GetData(ctx, c)
}
