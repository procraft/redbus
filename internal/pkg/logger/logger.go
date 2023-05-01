package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

var (
	Verbose bool
	JsonLog bool
	App     = context.Background()
)

type Level string

const (
	LevelDebug   Level = "D"
	LevelInfo    Level = "I"
	LevelWarning Level = "W"
	LevelError   Level = "E"
	LevelFatal   Level = "F"
)

var levelNameMap = map[Level]string{
	LevelDebug:   "debug",
	LevelInfo:    "info",
	LevelWarning: "warning",
	LevelError:   "error",
	LevelFatal:   "fatal",
}

type GetRequestIdFromContextGetterFn func(ctx context.Context) string

var GetRequestIdFromContextFn *GetRequestIdFromContextGetterFn

type jsonLog struct {
	Time     int64  `json:"time"`
	Request  string `json:"request"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func getRequestId(ctx context.Context) string {
	var v string
	if GetRequestIdFromContextFn != nil {
		v = (*GetRequestIdFromContextFn)(ctx)
	}
	if v == "" {
		return "<none>"
	}
	return v
}

func jsonPrintf(severity, requestId string, format string, v ...any) {
	bytes, err := json.Marshal(&jsonLog{time.Now().Unix(), requestId, severity, fmt.Sprintf(format, v...)})

	if err != nil {
		log.Printf("Logger failed to print message in JSON format")
		log.Printf("severity = %v", severity)
		log.Printf(format, v...)
		log.Fatal(err)
	}

	fmt.Printf("%s\n", string(bytes))
}

func Log(ctx context.Context, level Level, format string, v ...any) {
	if Verbose != true && level == LevelDebug {
		return
	}
	requestId := getRequestId(ctx)
	if JsonLog {
		jsonPrintf(levelNameMap[level], requestId, format, v...)
	} else {
		log.Printf("[%s] [%s] %s", level, requestId, fmt.Sprintf(format, v...))
	}
	if level == LevelFatal {
		os.Exit(1)
	}
}

func Debug(ctx context.Context, format string, v ...any) {
	Log(ctx, LevelDebug, format, v...)
}

func Info(ctx context.Context, format string, v ...any) {
	Log(ctx, LevelInfo, format, v...)
}

func Warning(ctx context.Context, format string, v ...any) {
	Log(ctx, LevelWarning, format, v...)
}

func Error(ctx context.Context, format string, v ...any) {
	Log(ctx, LevelError, format, v...)
}

func Fatal(ctx context.Context, format string, v ...any) {
	Log(ctx, LevelFatal, format, v...)
}
