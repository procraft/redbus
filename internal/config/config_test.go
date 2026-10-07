package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/procraft/redbus/internal/pkg/logger"
)

func TestFromFileAndEnvReadsAdminApiHost(t *testing.T) {
	t.Setenv("REDBUS_API_HOST", "https://redbus-api.sohoup.ru")

	conf, err := FromFileAndEnv(filepath.Join(t.TempDir(), "missing.json"))

	require.NoError(t, err)
	require.Equal(t, "https://redbus-api.sohoup.ru", conf.Admin.ApiHost)
}

func TestLokiConfigFromEnvWithProcessDefaults(t *testing.T) {
	t.Setenv("REDBUS_LOKI_URL", "http://loki:3100/loki/api/v1/push")
	t.Setenv("REDBUS_LOKI_USERNAME", "user")
	t.Setenv("REDBUS_LOKI_PASSWORD", "secret")
	t.Setenv("REDBUS_LOKI_ENV", "stage")

	conf, err := FromFileAndEnv(filepath.Join(t.TempDir(), "missing.json"))
	require.NoError(t, err)
	loki, err := conf.Log.Loki.Logger("redbus-admin")

	require.NoError(t, err)
	require.Equal(t, logger.LokiConfig{
		URL: "http://loki:3100/loki/api/v1/push", Username: "user", Password: "secret",
		App: "redbus-admin", Env: "stage", MinLevel: logger.LevelInfo,
	}, loki)

	t.Setenv("REDBUS_LOKI_APP", "bus")
	t.Setenv("REDBUS_LOKI_LEVEL", "loud")
	conf, err = FromFileAndEnv(filepath.Join(t.TempDir(), "missing.json"))
	require.NoError(t, err)
	_, err = conf.Log.Loki.Logger("redbus")
	require.ErrorContains(t, err, "log.loki.level")
}
