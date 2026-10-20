package migrator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDSNCarriesSSLSettings(t *testing.T) {
	require.Equal(t,
		"postgres://bus:p%40ss@db.example.com:6432/redbus?sslmode=require",
		dsn("db.example.com", 6432, "bus", "p@ss", "redbus", "require", ""))
	require.Equal(t,
		"postgres://bus:pw@db.example.com:6432/redbus?sslmode=verify-full&sslrootcert=%2Fetc%2Fssl%2Fca.crt",
		dsn("db.example.com", 6432, "bus", "pw", "redbus", "verify-full", "/etc/ssl/ca.crt"))
}
