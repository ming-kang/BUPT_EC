package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"BUPT_EC/cache"
	"BUPT_EC/config"
	"BUPT_EC/logs"
	"BUPT_EC/service"
	"BUPT_EC/utils"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type application struct {
	runtimeConfig    config.RuntimeConfig
	classroomService *service.ClassroomService
	httpServer       *HTTPServer
}

func Init() (*application, error) {
	runtimeConfig, err := config.Load(".env", os.LookupEnv)
	if err != nil {
		return nil, fmt.Errorf("load runtime config: %w", err)
	}

	gin.SetMode(runtimeConfig.GinMode)
	if err := logs.Init(true, runtimeConfig.LogCaller); err != nil {
		return nil, fmt.Errorf("init logging: %w", err)
	}

	store := cache.New()
	httpClient := utils.NewHTTPClient()
	jwClient, err := service.NewJWClient(runtimeConfig.JW.Username, runtimeConfig.JW.Password, httpClient)
	if err != nil {
		return nil, fmt.Errorf("create JW client: %w", err)
	}
	runtimeMetrics, err := service.NewPrometheusMetrics()
	if err != nil {
		return nil, fmt.Errorf("create runtime metrics: %w", err)
	}
	classroomService, err := service.NewClassroomService(service.ClassroomServiceOptions{
		Campuses:      runtimeConfig.Campuses,
		TokenOverride: runtimeConfig.JW.Token,
		Metrics:       runtimeMetrics,
	}, store, jwClient)
	if err != nil {
		return nil, fmt.Errorf("create classroom service: %w", err)
	}
	// DisableCompression: router gzipMiddleware is the sole Accept-Encoding owner.
	// Leaving promhttp compression on double-gzips responses and breaks scrapers.
	metricsHandler := promhttp.HandlerFor(runtimeMetrics.Registry(), promhttp.HandlerOpts{
		DisableCompression: true,
	})
	httpServer, err := NewHTTPServer(classroomService, runtimeConfig.HasJWCredentials, metricsHandler)
	if err != nil {
		return nil, fmt.Errorf("create HTTP server: %w", err)
	}

	return &application{
		runtimeConfig:    runtimeConfig,
		classroomService: classroomService,
		httpServer:       httpServer,
	}, nil
}

// httpWriteTimeout bounds how long the server may spend writing a response,
// including any handler wait. It must stay greater than
// service.ClassroomRefreshLimit so cold-path classroom refreshes that finish
// near the refresh budget are not cut off before JSON is written.
const httpWriteTimeout = 45 * time.Second

// gracefulShutdownTimeout covers HTTP draining plus any shared refresh worker
// that was already running when shutdown began.
const gracefulShutdownTimeout = httpWriteTimeout + 5*time.Second

func main() {
	app, err := Init()
	if err != nil {
		log.Fatalf("invalid startup configuration: %v", err)
	}

	appCtx, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()
	r := gin.New()
	r.Use(gin.Recovery())
	app.httpServer.RegisterRoutes(r)
	app.classroomService.StartWarmup(appCtx)
	addr := app.runtimeConfig.AppAddr

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
	defer signal.Stop(stop)

	var serveErr error
	select {
	case serveErr = <-serverErr:
	case sig := <-stop:
		log.Printf("received %s, shutting down", sig)
	}

	// Stop the scheduler first so it cannot add refresh workers while the HTTP
	// server and already-started background work are being drained.
	stopBackground()
	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server shutdown did not finish cleanly: %v", err)
	}
	if err := app.classroomService.WaitBackground(ctx); err != nil {
		log.Printf("background work did not finish before shutdown: %v", err)
	}
	if serveErr != nil {
		log.Fatalf("server failed: %v", serveErr)
	}
}
