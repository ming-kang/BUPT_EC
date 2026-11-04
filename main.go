package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"BUPT_EC/config"
	"BUPT_EC/logs"
	"BUPT_EC/service"
	"BUPT_EC/web"
)

// version is the build version reported by the startup log, /readyz and the
// API envelope. Release builds override it via -ldflags "-X main.version=<tag>".
var version = "dev"

// httpWriteTimeout bounds response writes including handler waits. The only
// waiting handler is /api/get_data, bounded by service.ColdWaitTimeout.
const httpWriteTimeout = 15 * time.Second

// shutdownTimeout covers HTTP draining after the refresh loop is canceled.
const shutdownTimeout = httpWriteTimeout + 5*time.Second

func main() {
	cfg, err := config.Load(".env", os.LookupEnv)
	if err != nil {
		log.Fatalf("invalid startup configuration: %v", err)
	}
	if err := logs.Init(true, cfg.LogCaller); err != nil {
		log.Fatalf("init logging: %v", err)
	}
	if _, embedded := web.Dist(); !embedded {
		slog.Warn("serving placeholder frontend (built without embed_assets)")
	}

	jwClient := service.NewJWClient(cfg.JWUsername, cfg.JWPassword, service.NewJWHTTPClient())
	classrooms := service.NewClassroomService(jwClient)

	// The refresh loop owns all JW work; canceling appCtx stops it and any
	// in-flight JW request.
	appCtx, stopLoop := context.WithCancel(context.Background())
	loopDone := make(chan struct{})
	go func() {
		classrooms.Run(appCtx)
		close(loopDone)
	}()

	server := &http.Server{
		Addr:              cfg.AppAddr,
		Handler:           NewHTTPServer(classrooms).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("BUPT_EC %s listening on %s", version, cfg.AppAddr)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	var serveErr error
	select {
	case serveErr = <-serverErr:
	case sig := <-stop:
		log.Printf("received %s, shutting down", sig)
	}

	stopLoop()
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server shutdown did not finish cleanly: %v", err)
	}
	select {
	case <-loopDone:
	case <-ctx.Done():
		log.Printf("refresh loop did not stop before shutdown deadline")
	}
	if serveErr != nil {
		log.Fatalf("server failed: %v", serveErr)
	}
}
