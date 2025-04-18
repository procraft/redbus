package reqid

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/prokraft/redbus/internal/pkg/logger"

	"github.com/go-chi/chi/v5/middleware"
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

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		return handler(SetRequestId(ctx), req)
	}
}

func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrapper := &streamWrapper{
			ctx:          SetRequestId(stream.Context()),
			ServerStream: stream,
		}
		return handler(srv, wrapper)
	}
}

func ServerMiddleware(extraPrefix ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(SetRequestId(r.Context(), extraPrefix...))
			next.ServeHTTP(w, r)
		})
	}
}

type streamWrapper struct {
	ctx context.Context
	grpc.ServerStream
}

func (w *streamWrapper) Context() context.Context {
	return w.ctx
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
	var getRequestIDFromContextFn logger.GetRequestIdFromContextGetterFn = func(ctx context.Context) string {
		if ctx == logger.App {
			return prefix + "/app"
		}
		return GetRequestId(ctx)
	}
	logger.GetRequestIdFromContextFn = &getRequestIDFromContextFn
}
