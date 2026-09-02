package adminapp

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type browserRuntimeConfig struct {
	ApiHost string `json:"apiHost"`
}

func runtimeConfigHandler(apiHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload, err := json.Marshal(browserRuntimeConfig{ApiHost: apiHost})
		if err != nil {
			http.Error(w, "encode runtime config", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = fmt.Fprintf(w, "window.__REDBUS_RUNTIME_CONFIG__ = %s;\n", payload)
	})
}
