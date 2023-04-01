package db

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/sergiusd/redbus/internal/pkg/logger"

	"github.com/sergiusd/redbus/internal/pkg/db/migrator"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

func New(ctx context.Context, host string, port int, user, password, name string, options ...Option) (IClient, error) {
	pool := DBPool{
		size: runtime.NumCPU()*2 + 2,
	}
	for _, o := range options {
		o(&pool)
	}

	dsnList := map[string]string{
		"host":           host,
		"port":           strconv.Itoa(port),
		"user":           user,
		"password":       password,
		"dbname":         name,
		"sslmode":        "disable",
		"pool_max_conns": strconv.Itoa(pool.size),
	}
	dsnStr := make([]string, len(dsnList))
	for k, v := range dsnList {
		if v != "" && v != "0" {
			dsnStr = append(dsnStr, k+"="+v)
		}
	}
	dsn := strings.Join(dsnStr, " ")

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("Unable to connect to database: %w", err)
	}

	dbLog := &dbLog{}
	if pool.log {
		config.ConnConfig.Logger = dbLog
	}

	pool.Pool, err = pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("Unable to connect to database: %w", err)
	}

	if pool.log {
		dbLog.db = pool.Pool
	}
	logger.Info(logger.App, "Use database %s@%s:%d/%s (conn size %v)", user, host, port, name, pool.size)

	// migrations
	mig, err := migrator.New(host, port, user, password, name)
	if err != nil {
		return nil, fmt.Errorf("Can't connect to database: %w", err)
	}
	if err := mig.Up(); err != nil {
		return nil, fmt.Errorf("Can't roll up migrations: %w", err)
	}
	if err := mig.Close(); err != nil {
		return nil, fmt.Errorf("Can't close migrate db connection: %w", err)
	}

	return &pool, nil
}

func ExecTx(ctx context.Context, fn func(ctx context.Context) error) error {
	conn := FromContext(ctx)
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	if err := fn(AddToContext(ctx, DBTx{tx})); err != nil {
		if err := tx.Rollback(ctx); err != nil {
			return err
		}
		return err
	}

	return tx.Commit(ctx)
}

type dbLog struct {
	db *pgxpool.Pool
}

func (l *dbLog) Log(ctx context.Context, level pgx.LogLevel, msg string, data map[string]interface{}) {
	var ac string
	if l.db != nil {
		ac = fmt.Sprintf("%v/%v/%v", l.db.Stat().AcquiredConns(), l.db.Stat().TotalConns(), l.db.Stat().MaxConns())
	}
	logger.Info(ctx, "[%v] %v %v %v", ac, level, data, msg)
}
