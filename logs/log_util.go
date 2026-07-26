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

	"gopkg.in/natefinch/lumberjack.v2"
)

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
