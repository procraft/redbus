package db

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type IClient interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error

	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

// DB POOL

type DBPool struct {
	*pgxpool.Pool
	size int
	log  bool
}

func (d *DBPool) Commit(ctx context.Context) error {
	return nil
}
func (d *DBPool) Rollback(ctx context.Context) error {
	return nil
}

// DB TRANSACTION

type DBTx struct {
	pgx.Tx
}

func (tx DBTx) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	return tx.Tx, nil
}
