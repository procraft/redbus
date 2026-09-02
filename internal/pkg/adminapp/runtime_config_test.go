package adminapp

import (
	"net/http/httptest"
	"testing"

	"github.com/prokraft/redbus/internal/config"

	"github.com/stretchr/testify/require"
)

func TestNewRequiresRuntimeApiHost(t *testing.T) {
	app, err := New(&config.Config{})

	require.Nil(t, app)
	require.EqualError(t, err, "REDBUS_API_HOST is required")
}

func TestRuntimeConfigHandler(t *testing.T) {
	request := httptest.NewRequest("GET", "/runtime-config.js", nil)
	response := httptest.NewRecorder()

	runtimeConfigHandler(`https://redbus-api.sohoup.ru/</script><script>alert(1)</script>`).
		ServeHTTP(response, request)

	require.Equal(t, "application/javascript; charset=utf-8", response.Header().Get("Content-Type"))
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Equal(
		t,
		"window.__REDBUS_RUNTIME_CONFIG__ = {\"apiHost\":\"https://redbus-api.sohoup.ru/\\u003c/script\\u003e\\u003cscript\\u003ealert(1)\\u003c/script\\u003e\"};\n",
		response.Body.String(),
	)
}
