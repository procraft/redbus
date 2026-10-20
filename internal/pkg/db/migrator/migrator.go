package migrator

import (
	"database/sql"
	"errors"
	"net"
	"net/url"
	"strconv"

	"github.com/golang-migrate/migrate/v4"

	"github.com/procraft/redbus/internal/pkg/logger"

	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

func New(host string, port int, user, password, name, sslMode, sslRootCert string) (*Migrator, error) {
	db, err := sql.Open("postgres", dsn(host, port, user, password, name, sslMode, sslRootCert))
	if err != nil {
		return nil, err
	}
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, err
	}
	return &Migrator{
		driver: driver,
		dir:    "internal/migrations",
	}, nil
}

// dsn builds the lib/pq URL; sslmode follows the main pool so migrations never run without TLS
// when the pool requires it.
func dsn(host string, port int, user, password, name, sslMode, sslRootCert string) string {
	query := url.Values{}
	query.Set("sslmode", sslMode)
	if sslRootCert != "" {
		query.Set("sslrootcert", sslRootCert)
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		Path:     "/" + name,
		RawQuery: query.Encode(),
	}
	return u.String()
}

type Migrator struct {
	driver database.Driver
	dir    string
}

func (m *Migrator) Up() error {
	logger.Info(logger.App, "Rolling up migration from dir: %v", m.dir)
	mig, err := migrate.NewWithDatabaseInstance("file://"+m.dir, "postgres", m.driver)
	if err != nil {
		return err
	}
	err = mig.Up()
	if err == nil || errors.Is(err, migrate.ErrNoChange) {
		version, dirty, migErr := mig.Version()
		if migErr != nil {
			logger.Fatal(logger.App, "Can't get database version %v", migErr)
			return migErr
		}
		if err == nil {
			logger.Info(logger.App, "Database up to version %v, dirty = %v", version, dirty)
		} else {
			logger.Info(logger.App, "Database doesn't change: version %v, dirty = %v", version, dirty)
		}
		return nil
	}
	return err
}

func (m *Migrator) Close() error {
	return m.driver.Close()
}
