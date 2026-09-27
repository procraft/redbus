// Package pgtest gives SDK tests a throwaway PostgreSQL database with the client tables from
// api/inbox.sql and api/outbox.sql.
//
// The tests are opt-in: set REDBUS_PG_TEST_URL to a server URL whose user may create databases,
// e.g. postgres://localhost:5432/postgres?sslmode=disable. Each test gets its own database, which
// is dropped afterwards.
package pgtest

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

const EnvURL = "REDBUS_PG_TEST_URL"

// DB is a throwaway database; DSN is its URL for extra connections (e.g. LISTEN).
type DB struct {
	*sql.DB
	DSN string
}

// New creates the database with the client tables, or skips the test without REDBUS_PG_TEST_URL.
func New(t *testing.T) DB {
	t.Helper()
	serverURL := os.Getenv(EnvURL)
	if serverURL == "" {
		t.Skipf("set %s to run PostgreSQL tests", EnvURL)
	}
	admin, err := sql.Open("postgres", serverURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	name := "redbus_sdk_test_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("create test database: %v", err)
	}

	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	dsn := u.String()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		if _, err := admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`); err != nil {
			t.Errorf("drop test database %s: %v", name, err)
		}
	})
	for _, file := range []string{"inbox.sql", "outbox.sql"} {
		if _, err := db.Exec(ups(t, file)); err != nil {
			t.Fatalf("apply %s: %v", file, err)
		}
	}
	return DB{DB: db, DSN: dsn}
}

// ups returns the "# --- !Ups" section of a schema file in api/.
func ups(t *testing.T, file string) string {
	_, self, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(self), "..", "..", "..", file))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if i := strings.Index(s, "# --- !Downs"); i >= 0 {
		s = s[:i]
	}
	return strings.Replace(s, "# --- !Ups", "", 1)
}
