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
