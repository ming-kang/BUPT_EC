package main

import (
	"BUPT_EC/cache"
	"BUPT_EC/config"
	"BUPT_EC/logs"
	"log"
)

func Init() {
	logs.Init(true)
	config.InitConfig()
	if err := config.ValidateRuntimeConfig(); err != nil {
		log.Fatalf("invalid runtime config: %v", err)
	}
	cache.InitCache()
}
