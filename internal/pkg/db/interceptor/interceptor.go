package interceptor

import (
	"context"
	"net/http"

	"github.com/prokraft/redbus/internal/pkg/db"

	"google.golang.org/grpc"
)

// DBClientProvider is a func for wrapping database
type DBClientProvider func(ctx context.Context) db.IClient

// UnaryServerInterceptor wrap endpoint with middleware mixing in db connection
func UnaryServerInterceptor(dbClientProvider DBClientProvider, option ...db.Option) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		return handler(db.AddToContext(ctx, dbClientProvider(ctx)), req)
	}
}

// UnaryServerInterceptor wrap endpoint with middleware mixing in db connection
func StreamServerInterceptor(dbClientProvider DBClientProvider, option ...db.Option) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := stream.Context()
		wrapper := &streamWrapper{
			ctx:          db.AddToContext(ctx, dbClientProvider(ctx)),
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

// ServerMiddleware wrap endpoint with middleware mixing in db connection
func ServerMiddleware(dbClientProvider DBClientProvider, option ...db.Option) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			r = r.WithContext(db.AddToContext(ctx, dbClientProvider(ctx)))
			next.ServeHTTP(w, r)
		})
	}
}
