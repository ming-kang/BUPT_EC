package logs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogIDKey is retained for callers that need a stable string label; the
// context value itself uses an unexported typed key to avoid collisions.
const LogIDKey = "K_LOGID"

type ctxKey int

const logIDCtxKey ctxKey = 1

// mainLogDir is the production log directory relative to the process working
// directory. Tests may override it via setMainLogDirForTest.
var mainLogDir = "run_log"

// Init configures the process logger. It never exits the process; callers must
// handle the returned error at the composition root.
func Init(isMain, addSource bool) error {
	var writer io.Writer
	if isMain {
		if err := os.MkdirAll(mainLogDir, 0750); err != nil {
			return fmt.Errorf("create log directory: %w", err)
		}
		fileWriter := &lumberjack.Logger{
			Filename:   filepath.Join(mainLogDir, "ec.log"),
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
		Level:     slog.LevelInfo,
		AddSource: addSource,
	}

	baseHandler := slog.NewJSONHandler(writer, opts)
	logger := slog.New(&logIDHandler{Handler: baseHandler})
	slog.SetDefault(logger)
	return nil
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
	return context.WithValue(parent, logIDCtxKey, GenLogID())
}

func GetLogIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	v := ctx.Value(logIDCtxKey)
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
