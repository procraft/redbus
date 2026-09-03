package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/db"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

type execRecorder struct {
	sql       string
	arguments []interface{}
}

func (r *execRecorder) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	panic("unexpected BeginTx call")
}

func (r *execRecorder) Commit(context.Context) error {
	panic("unexpected Commit call")
}

func (r *execRecorder) Rollback(context.Context) error {
	panic("unexpected Rollback call")
}

func (r *execRecorder) Exec(_ context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	r.sql = sql
	r.arguments = arguments
	return pgconn.CommandTag{}, nil
}

func (r *execRecorder) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	panic("unexpected Query call")
}

func (r *execRecorder) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	panic("unexpected QueryRow call")
}

func TestRepeatFieldsMatchScanDestinations(t *testing.T) {
	fields := strings.Split(repeatFields, ",")
	destinations := repeatScanDest(&model.Repeat{})

	if len(fields) != len(destinations) {
		t.Fatalf("repeat field count must match scan destinations: fields=%d destinations=%d", len(fields), len(destinations))
	}
}

func TestRestartFailedByErrorFiltersSelectedError(t *testing.T) {
	client := &execRecorder{}
	ctx := db.AddToContext(context.Background(), client)
	since := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	err := (&Repository{}).RestartFailedByError(ctx, "orders", "billing", "invalid link", since)
	if err != nil {
		t.Fatalf("restart failed by error: %v", err)
	}

	normalizedSQL := strings.Join(strings.Fields(client.sql), " ")
	if !strings.Contains(normalizedSQL, `WHERE finished_at IS NOT NULL AND topic = $2 AND "group" = $3 AND error = $4 AND finished_at >= $5`) {
		t.Fatalf("restart must filter by the selected error, query: %s", normalizedSQL)
	}
	if len(client.arguments) != 5 {
		t.Fatalf("unexpected argument count: got %d want 5", len(client.arguments))
	}
	if client.arguments[1] != "orders" || client.arguments[2] != "billing" || client.arguments[3] != "invalid link" || client.arguments[4] != since {
		t.Fatalf("unexpected restart filters: %#v", client.arguments[1:])
	}
}

func TestRestartFailedSinceFiltersPeriod(t *testing.T) {
	client := &execRecorder{}
	ctx := db.AddToContext(context.Background(), client)
	since := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	err := (&Repository{}).RestartFailedSince(ctx, "orders", "billing", since)
	if err != nil {
		t.Fatalf("restart failed since: %v", err)
	}

	normalizedSQL := strings.Join(strings.Fields(client.sql), " ")
	if !strings.Contains(normalizedSQL, `WHERE finished_at IS NOT NULL AND topic = $2 AND "group" = $3 AND finished_at >= $4`) {
		t.Fatalf("restart must filter by period, query: %s", normalizedSQL)
	}
	if len(client.arguments) != 4 {
		t.Fatalf("unexpected argument count: got %d want 4", len(client.arguments))
	}
	if client.arguments[1] != "orders" || client.arguments[2] != "billing" || client.arguments[3] != since {
		t.Fatalf("unexpected restart filters: %#v", client.arguments[1:])
	}
}
