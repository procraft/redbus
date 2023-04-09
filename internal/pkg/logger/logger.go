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
	Jsonlog bool
	App     = "app"
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
type GetRequestIdFromStringGetterFn func(ctx string) string

var GetRequestIdFromContextFn *GetRequestIdFromContextGetterFn
var GetRequestIdFromStringFn *GetRequestIdFromStringGetterFn

type jsonLog struct {
	Time     int64  `json:"time"`
	Request  string `json:"request"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func getRequestId(id any) string {
	switch v := id.(type) {
	case string:
		if GetRequestIdFromStringFn != nil {
			return (*GetRequestIdFromStringFn)(v)
		}
		return v
	case context.Context:
		if GetRequestIdFromContextFn != nil {
			return (*GetRequestIdFromContextFn)(v)
		}
	}
	return "<none>"
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

func Log(ctx any, level Level, format string, v ...any) {
	if Verbose != true && level == LevelDebug {
		return
	}
	requestId := getRequestId(ctx)
	if Jsonlog {
		jsonPrintf(levelNameMap[level], requestId, format, v...)
	} else {
		log.Printf("[%s] [%s] %s", level, requestId, fmt.Sprintf(format, v...))
	}
	if level == LevelFatal {
		os.Exit(1)
	}
}

func Debug(ctx any, format string, v ...any) {
	Log(ctx, LevelDebug, format, v...)
}

func Info(ctx any, format string, v ...any) {
	Log(ctx, LevelInfo, format, v...)
}

func Warning(ctx any, format string, v ...any) {
	Log(ctx, LevelWarning, format, v...)
}

func Error(ctx any, format string, v ...any) {
	Log(ctx, LevelError, format, v...)
}

func Fatal(ctx any, format string, v ...any) {
	Log(ctx, LevelFatal, format, v...)
}
