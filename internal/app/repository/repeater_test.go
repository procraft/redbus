package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/db"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgx/v4"
)

type execRecorder struct {
	sql       string
	arguments []interface{}

	storedErrors   []string
	queryArguments []interface{}
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

func (r *execRecorder) Query(_ context.Context, _ string, arguments ...interface{}) (pgx.Rows, error) {
	r.queryArguments = arguments
	return &stringRows{values: r.storedErrors, index: -1}, nil
}

type stringRows struct {
	values []string
	index  int
}

func (r *stringRows) Close()                                         {}
func (r *stringRows) Err() error                                     { return nil }
func (r *stringRows) CommandTag() pgconn.CommandTag                  { return nil }
func (r *stringRows) FieldDescriptions() []pgproto3.FieldDescription { return nil }
func (r *stringRows) Values() ([]interface{}, error)                 { return nil, nil }
func (r *stringRows) RawValues() [][]byte                            { return nil }

func (r *stringRows) Next() bool {
	r.index++
	return r.index < len(r.values)
}

func (r *stringRows) Scan(dest ...interface{}) error {
	*dest[0].(*string) = r.values[r.index]
	return nil
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

func TestRepeatTriageSQLUsesBoundedFinishedWindowAndOptionalFilters(t *testing.T) {
	normalizedSQL := strings.Join(strings.Fields(repeatTriageSQL), " ")
	for _, clause := range []string{
		`finished_at >= $1`,
		`finished_at < $2`,
		`($3 = '' OR topic = $3)`,
		`($4 = '' OR "group" = $4)`,
		`GROUP BY topic, "group", error`,
	} {
		if !strings.Contains(normalizedSQL, clause) {
			t.Fatalf("triage query must contain %q: %s", clause, normalizedSQL)
		}
	}
}

func TestRestartFailedByErrorFiltersErrorClass(t *testing.T) {
	client := &execRecorder{storedErrors: []string{"task 1 has no message", "invalid link", "task 22 has no message"}}
	ctx := db.AddToContext(context.Background(), client)
	since := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	err := (&Repository{}).RestartFailedByError(ctx, "orders", "billing", "task <N> has no message", since)
	if err != nil {
		t.Fatalf("restart failed by error: %v", err)
	}

	if len(client.queryArguments) != 3 || client.queryArguments[2].(*time.Time) == nil || !client.queryArguments[2].(*time.Time).Equal(since) {
		t.Fatalf("stored errors must be resolved inside the restart period: %#v", client.queryArguments)
	}
	normalizedSQL := strings.Join(strings.Fields(client.sql), " ")
	if !strings.Contains(normalizedSQL, `WHERE finished_at IS NOT NULL AND topic = $2 AND "group" = $3 AND error = ANY($4) AND finished_at >= $5`) {
		t.Fatalf("restart must filter by the selected error class, query: %s", normalizedSQL)
	}
	if len(client.arguments) != 5 {
		t.Fatalf("unexpected argument count: got %d want 5", len(client.arguments))
	}
	if client.arguments[1] != "orders" || client.arguments[2] != "billing" || client.arguments[4] != since {
		t.Fatalf("unexpected restart filters: %#v", client.arguments[1:])
	}
	errors := client.arguments[3].([]string)
	if len(errors) != 2 || errors[0] != "task 1 has no message" || errors[1] != "task 22 has no message" {
		t.Fatalf("restart must target exact messages of the class: %#v", errors)
	}
}

func TestRestartFailedByErrorSkipsWriteWithoutMatchingErrors(t *testing.T) {
	client := &execRecorder{storedErrors: []string{"invalid link"}}
	ctx := db.AddToContext(context.Background(), client)

	err := (&Repository{}).RestartFailedByError(ctx, "orders", "billing", "task <N> has no message", time.Now())
	if err != nil {
		t.Fatalf("restart failed by error: %v", err)
	}
	if client.sql != "" {
		t.Fatalf("restart without matching errors must not write: %s", client.sql)
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

func TestDeleteFailedByErrorDeletesOnlyFinishedRepeatsOfClass(t *testing.T) {
	client := &execRecorder{storedErrors: []string{"task 1 has no message", "invalid link"}}
	ctx := db.AddToContext(context.Background(), client)

	err := (&Repository{}).DeleteFailedByError(ctx, "orders", "billing", "task 7 has no message")
	if err != nil {
		t.Fatalf("delete failed by error: %v", err)
	}

	if len(client.queryArguments) != 3 || client.queryArguments[2].(*time.Time) != nil {
		t.Fatalf("delete must resolve stored errors without a period: %#v", client.queryArguments)
	}
	normalizedSQL := strings.Join(strings.Fields(client.sql), " ")
	if !strings.Contains(normalizedSQL, `DELETE FROM repeat WHERE finished_at IS NOT NULL AND topic = $1 AND "group" = $2 AND error = ANY($3)`) {
		t.Fatalf("delete must affect only finished repeats of the selected error class, query: %s", normalizedSQL)
	}
	if len(client.arguments) != 3 {
		t.Fatalf("unexpected argument count: got %d want 3", len(client.arguments))
	}
	errors := client.arguments[2].([]string)
	if client.arguments[0] != "orders" || client.arguments[1] != "billing" || len(errors) != 1 || errors[0] != "task 1 has no message" {
		t.Fatalf("unexpected delete filters: %#v", client.arguments)
	}
}
