package config

import (
	"os"
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

func TestDBSSLModeDefaultsToRequire(t *testing.T) {
	conf, err := FromFileAndEnv(filepath.Join(t.TempDir(), "missing.json"))

	require.NoError(t, err)
	mode, rootCert := conf.DB.SSL()
	require.Equal(t, "require", mode)
	require.Empty(t, rootCert)
}

func TestDBSSLModeFromEnv(t *testing.T) {
	t.Setenv("REDBUS_DB_SSLMODE", "verify-full")
	t.Setenv("REDBUS_DB_SSLROOTCERT", "/etc/ssl/db-ca.crt")

	conf, err := FromFileAndEnv(filepath.Join(t.TempDir(), "missing.json"))

	require.NoError(t, err)
	mode, rootCert := conf.DB.SSL()
	require.Equal(t, "verify-full", mode)
	require.Equal(t, "/etc/ssl/db-ca.crt", rootCert)
}

func TestDBSSLModeRejectsUnknownValue(t *testing.T) {
	t.Setenv("REDBUS_DB_SSLMODE", "prefer")

	_, err := FromFileAndEnv(filepath.Join(t.TempDir(), "missing.json"))

	require.ErrorContains(t, err, "REDBUS_DB_SSLMODE")
}

func TestRepoConfigDisablesDBSSLForLocalDevelopment(t *testing.T) {
	conf, err := FromFileAndEnv(filepath.Join("..", "..", "config.json"))

	require.NoError(t, err)
	mode, _ := conf.DB.SSL()
	require.Equal(t, "disable", mode)
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

func TestKafkaMessageIdNamespaceFromFileAndEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("REDBUS_KAFKA_MESSAGE_ID_NAMESPACE", "")
	conf, err := FromFileAndEnv(path)
	require.NoError(t, err)
	require.Empty(t, conf.Kafka.MessageIdNamespace)

	require.NoError(t, os.WriteFile(path, []byte(`{"kafka":{"messageIdNamespace":"file-generation"}}`), 0600))
	conf, err = FromFileAndEnv(path)
	require.NoError(t, err)
	require.Equal(t, "file-generation", conf.Kafka.MessageIdNamespace)

	t.Setenv("REDBUS_KAFKA_MESSAGE_ID_NAMESPACE", "env-generation")
	conf, err = FromFileAndEnv(path)
	require.NoError(t, err)
	require.Equal(t, "env-generation", conf.Kafka.MessageIdNamespace)
}

func TestDBAddress(t *testing.T) {
	cases := []struct {
		host     string
		wantHost string
		wantPort int
	}{
		{"db.example.com", "db.example.com", 6432},
		{"db.example.com:5432", "db.example.com", 5432},
		{"[::1]:5432", "::1", 5432},
	}
	for _, c := range cases {
		t.Run(c.host, func(t *testing.T) {
			t.Setenv("REDBUS_DB_HOST", c.host)
			t.Setenv("REDBUS_DB_PORT", "6432")

			conf, err := FromFileAndEnv(filepath.Join(t.TempDir(), "missing.json"))

			require.NoError(t, err)
			host, port, err := conf.DB.Address()
			require.NoError(t, err)
			require.Equal(t, c.wantHost, host)
			require.Equal(t, c.wantPort, port)
		})
	}
}

func TestDBAddressRejectsInvalidPort(t *testing.T) {
	t.Setenv("REDBUS_DB_HOST", "db.example.com:abc")

	_, err := FromFileAndEnv(filepath.Join(t.TempDir(), "missing.json"))

	require.ErrorContains(t, err, "REDBUS_DB_HOST")
}
