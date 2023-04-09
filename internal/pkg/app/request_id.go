package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/sergiusd/redbus/internal/pkg/logger"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/middleware"
	"google.golang.org/grpc"
)

const requestIDHeaderName = "X-Request-Id"

var prefix string

func GetRequestIdPrefix(extraPrefix ...string) string {
	extraStr := ""
	if len(extraPrefix) != 0 {
		extraStr = "/" + strings.Join(extraPrefix, "/")
	}
	return fmt.Sprintf("%s%s", prefix, extraStr)
}

func SetRequestId(ctx context.Context, extraPrefix ...string) context.Context {
	id := middleware.NextRequestID()
	return context.WithValue(ctx, middleware.RequestIDKey, fmt.Sprintf("%s-%06d", GetRequestIdPrefix(extraPrefix...), id))
}

func GetRequestId(ctx context.Context) string {
	return middleware.GetReqID(ctx)
}

func NewRequestIdInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		return handler(SetRequestId(ctx), req)
	}
}

func NewRequestIdMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if key, ok := r.Context().Value(middleware.RequestIDKey).(string); ok {
				w.Header().Set(requestIDHeaderName, key)
			}
			next.ServeHTTP(w, r)
		}
		return middleware.RequestID(http.HandlerFunc(fn))
	}
}

func init() {
	hostname, err := os.Hostname()
	if hostname == "" || err != nil {
		hostname = "localhost"
	}
	var buf [12]byte
	var b64 string
	for len(b64) < 10 {
		_, _ = rand.Read(buf[:])
		b64 = base64.StdEncoding.EncodeToString(buf[:])
		b64 = strings.NewReplacer("+", "", "/", "").Replace(b64)
	}

	prefix = fmt.Sprintf("%s/%s", hostname, b64[0:10])

	// bind with logger
	var getRequestIDFromStringFn logger.GetRequestIdFromStringGetterFn = func(ctx string) string {
		return prefix + "/" + ctx
	}
	logger.GetRequestIdFromStringFn = &getRequestIDFromStringFn
	var getRequestIDFromContextFn logger.GetRequestIdFromContextGetterFn = func(ctx context.Context) string {
		return GetRequestId(ctx)
	}
	logger.GetRequestIdFromContextFn = &getRequestIDFromContextFn
}
