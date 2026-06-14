package main

import (
	"BUPT_EC/cache"
	"BUPT_EC/config"
	"BUPT_EC/logs"
)

func Init() {
	logs.Init(true)
	config.InitConfig()
	cache.InitCache()
}
