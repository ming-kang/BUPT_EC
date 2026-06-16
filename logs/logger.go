package logs

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"sync"
)

var (
	defaultLogger   *log.Logger
	defaultLoggerMu sync.RWMutex
	callerEnabled   bool
)

func GetCallerInfo() (info string) {
	_, file, lineNo, ok := runtime.Caller(2)
	if !ok {
		info = "runtime.Caller() failed"
		return
	}
	fileName := path.Base(file) // Base函数返回路径的最后一个元素
	return fmt.Sprintf("%s:%d", fileName, lineNo)
}

func CtxInfo(ctx context.Context, format string, v ...interface{}) {
	writeLog(ctx, "INFO", format, v...)
}

func CtxWarn(ctx context.Context, format string, v ...interface{}) {
	writeLog(ctx, "WARN", format, v...)
}

func CtxError(ctx context.Context, format string, v ...interface{}) {
	writeLog(ctx, "ERROR", format, v...)
}

func writeLog(ctx context.Context, level string, format string, v ...interface{}) {
	defaultLoggerMu.RLock()
	logger := defaultLogger
	includeCaller := callerEnabled
	defaultLoggerMu.RUnlock()

	if logger == nil {
		defaultLoggerMu.Lock()
		if defaultLogger == nil {
			defaultLogger = log.New(os.Stdout, "", log.Ldate|log.Lmicroseconds)
		}
		logger = defaultLogger
		includeCaller = callerEnabled
		defaultLoggerMu.Unlock()
	}

	logID := GetLogIDFromContext(ctx)
	if includeCaller {
		logger.Printf("[%s] %s %s "+format, append([]interface{}{level, GetCallerInfo(), logID}, v...)...)
		return
	}
	logger.Printf("[%s] %s "+format, append([]interface{}{level, logID}, v...)...)
}
