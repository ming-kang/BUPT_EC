package main

import (
	"BUPT_EC/cache"
	"BUPT_EC/config"
	"BUPT_EC/logs"
	"BUPT_EC/service"
	"log"
)

var classroomService *service.ClassroomService

func Init() {
	logs.Init(true)
	config.InitConfig()
	if err := config.ValidateRuntimeConfig(); err != nil {
		log.Fatalf("invalid runtime config: %v", err)
	}
	cache.InitCache()
	classroomService = service.NewClassroomService(config.GetConfig(), cache.GlobalCache)
}
