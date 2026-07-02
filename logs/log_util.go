package logs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	LogIDKey = "K_LOGID"
)

func Init(isMain bool) {
	var writer io.Writer
	if isMain {
		if err := os.MkdirAll("run_log", 0750); err != nil {
			log.Fatalf("create log directory failed: %v", err)
		}
		fileWriter := &lumberjack.Logger{
			Filename:   "run_log/ec.log",
			MaxSize:    10,
			MaxBackups: 5,
			MaxAge:     30,
			Compress:   true,
		}
		writer = io.MultiWriter(os.Stdout, fileWriter)
	} else {
		writer = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if os.Getenv("LOG_CALLER") == "1" || strings.EqualFold(os.Getenv("LOG_CALLER"), "true") {
		opts.AddSource = true
	}

	baseHandler := slog.NewJSONHandler(writer, opts)
	logger := slog.New(&logIDHandler{Handler: baseHandler})
	slog.SetDefault(logger)
}

func SetNewContextForGinContext(c *gin.Context) {
	newCtx := GenNewContext(c.Request.Context())
	c.Set("ctx", newCtx)
	c.Writer.Header().Set("LogID", GetLogIDFromContext(newCtx))
}

func RandomHex(n int) string {
	bytes := make([]byte, n)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func GenLogID() string {
	return time.Now().Format("20060102150405") + strings.ToUpper(RandomHex(9))
}

func GenNewContext(parent context.Context) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithValue(parent, LogIDKey, GenLogID())
}

func GetLogIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	v := ctx.Value(LogIDKey)
	if v == nil {
		return ""
	}

	logID, ok := v.(string)
	if !ok {
		return ""
	}
	return logID
}

func GetContextFromGinContext(c *gin.Context) context.Context {
	if c == nil || c.Request == nil {
		return GenNewContext(context.Background())
	}
	if raw, ok := c.Get("ctx"); ok {
		if ctx, ok := raw.(context.Context); ok {
			return ctx
		}
	}
	return GenNewContext(c.Request.Context())
}
