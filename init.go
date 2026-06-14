package main

import (
	"EmptyClassroom/cache"
	"EmptyClassroom/config"
	"EmptyClassroom/logs"
)

func Init() {
	logs.Init(true)
	config.InitConfig()
	cache.InitCache()
}
