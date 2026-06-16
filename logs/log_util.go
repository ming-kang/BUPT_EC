package logs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	LogIDKey = "K_LOGID"
)

func Init(isMain bool) {
	defaultLoggerMu.Lock()
	defer defaultLoggerMu.Unlock()
	callerEnabled = os.Getenv("LOG_CALLER") == "1" || strings.EqualFold(os.Getenv("LOG_CALLER"), "true")
	if isMain {
		stdoutWriter := os.Stdout
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
		defaultLogger = log.New(io.MultiWriter(stdoutWriter, fileWriter), "", log.Ldate|log.Lmicroseconds)
	} else {
		stdoutWriter := os.Stdout
		defaultLogger = log.New(io.MultiWriter(stdoutWriter), "", log.Ldate|log.Lmicroseconds)
	}
}

func SetNewContextForGinContext(c *gin.Context) {
	newCtx := GenNewContext(c.Request.Context())
	c.Set("ctx", newCtx)
	c.Writer.Header().Set("LogID", GetLogIDFromContext(newCtx))
}

func FillZeroForInt(i int, w int) string {
	rawStr := fmt.Sprintf("%d", i)
	for len(rawStr) < w {
		rawStr = "0" + rawStr
	}
	return rawStr
}

func RandomHex(n int) string {
	bytes := make([]byte, n)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)

}

func GenLogID() string {
	str := ""
	t := time.Now()
	year, month, day := t.Date()
	hour, minute, second := t.Clock()
	str += FillZeroForInt(year, 4)
	str += FillZeroForInt(int(month), 2)
	str += FillZeroForInt(day, 2)
	str += FillZeroForInt(hour, 2)
	str += FillZeroForInt(minute, 2)
	str += FillZeroForInt(second, 2)
	str += strings.ToUpper(RandomHex(9))
	return str
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
