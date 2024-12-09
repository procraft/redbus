package adminapi

import (
	"net/http"
	"strings"
)

func AuthMiddleware(token string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			value := r.Header.Get("Authorization")
			parts := strings.Split(value, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "token" || parts[1] != token {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("Wrong authenticate token"))
				return
			}

			next.ServeHTTP(w, r)

		})
	}
}
