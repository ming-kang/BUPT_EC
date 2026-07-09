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

func Init() *service.ClassroomService {
	logs.Init(true)
	config.InitConfig()
	if err := config.ValidateRuntimeConfig(); err != nil {
		log.Fatalf("invalid runtime config: %v", err)
	}
	cache.InitCache()
	return service.NewClassroomService(config.GetConfig(), cache.GlobalCache)
}

// httpWriteTimeout bounds how long the server may spend writing a response,
// including any handler wait. It must stay greater than
// service.ClassroomRefreshLimit so cold-path classroom refreshes that finish
// near the refresh budget are not cut off before JSON is written.
const httpWriteTimeout = 45 * time.Second

// listenAddr returns the HTTP listen address from APP_ADDR.
// When env is empty, default to loopback so an unbound process is not
// reachable on all interfaces.
func listenAddr(env string) string {
	if env == "" {
		return "127.0.0.1:8080"
	}
	return env
}

func main() {
	classroomService := Init()
	r := gin.New()
	r.Use(gin.Recovery())
	httpServer := NewHTTPServer(classroomService, config.HasJWCredentials)
	httpServer.RegisterRoutes(r)
	classroomService.StartWarmup()
	addr := listenAddr(os.Getenv("APP_ADDR"))

	server := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// Must exceed service.ClassroomRefreshLimit: cold /api/get_data waits for a
		// shared refresh (up to that budget) before writing JSON. Keep margin for
		// response serialization after a near-limit refresh.
		WriteTimeout:   httpWriteTimeout,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
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
