package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
