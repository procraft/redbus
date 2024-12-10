package migrator

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"

	"github.com/prokraft/redbus/internal/pkg/logger"

	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

func New(host string, port int, user, password, name string) (*Migrator, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", user, password, host, port, name)
	db, err := sql.Open("postgres", dsn)
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
