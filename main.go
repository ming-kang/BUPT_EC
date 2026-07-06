package main

import (
	"BUPT_EC/cache"
	"BUPT_EC/config"
	"BUPT_EC/logs"
	"BUPT_EC/service"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
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

func main() {
	Init()
	r := gin.New()
	r.Use(gin.Recovery())
	SetRouter(r)
	classroomService.StartWarmup()
	addr := os.Getenv("APP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("BUPT_EC listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("server failed: %v", err)
		}
	case sig := <-stop:
		log.Printf("received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("server shutdown failed: %v", err)
		}
		if err := classroomService.WaitWarmup(ctx); err != nil {
			log.Printf("background refresh did not finish before shutdown: %v", err)
		}
	}
}
