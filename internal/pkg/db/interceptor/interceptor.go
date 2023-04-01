package interceptor

import (
	"context"
	"net/http"

	"github.com/sergiusd/redbus/internal/pkg/db"

	"google.golang.org/grpc"
)

// DBClientProvider is a func for wrapping database
type DBClientProvider func(ctx context.Context) db.IClient

// NewDBInterceptor wrap endpoint with middleware mixing in db connection
func NewDBInterceptor(dbClientProvider DBClientProvider, option ...db.Option) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		return handler(db.AddToContext(ctx, dbClientProvider(ctx)), req)
	}
}

// NewDBServerMiddleware wrap endpoint with middleware mixing in db connection
func NewDBServerMiddleware(dbClientProvider DBClientProvider, option ...db.Option) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			r = r.WithContext(db.AddToContext(ctx, dbClientProvider(ctx)))
			next.ServeHTTP(w, r)
		})
	}
}
