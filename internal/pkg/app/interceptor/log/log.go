package log

import (
	"context"
	"github.com/prokraft/redbus/internal/pkg/logger"
	"google.golang.org/grpc"
	"net/http"
)

// TODO
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		return handler(ctx, req)
	}
}

// TODO
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrapper := &streamWrapper{
			ctx:          stream.Context(),
			ServerStream: stream,
		}
		return handler(srv, wrapper)
	}
}

type streamWrapper struct {
	ctx context.Context
	grpc.ServerStream
}

func (w *streamWrapper) Context() context.Context {
	return w.ctx
}

func ServerMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapper := &writerWrapper{
				ResponseWriter: w,
				statusCode:     200,
			}
			next.ServeHTTP(wrapper, r)
			if wrapper.statusCode != 200 {
				logger.Error(r.Context(), "Error on handle HTTP %v: [%v] %v", r.URL.Path, wrapper.statusCode, string(wrapper.data))
			}
		})
	}
}

type writerWrapper struct {
	http.ResponseWriter
	statusCode int
	data       []byte
}

func (w *writerWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *writerWrapper) Write(b []byte) (int, error) {
	w.data = append(w.data, b...)
	return w.ResponseWriter.Write(b)
}
