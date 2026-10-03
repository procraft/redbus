package repository

import (
	"context"
	"fmt"
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

func TestRepeatCountSQLSeparatesDeferredFromFailed(t *testing.T) {
	normalizedSQL := strings.Join(strings.Fields(repeatCountSQL), " ")
	for _, clause := range []string{
		`COUNT(*) FILTER (WHERE finished_at IS NOT NULL) AS failed_count`,
		`COUNT(*) FILTER (WHERE finished_at IS NULL AND deferred) AS deferred_count`,
	} {
		if !strings.Contains(normalizedSQL, clause) {
			t.Fatalf("count query must contain %q: %s", clause, normalizedSQL)
		}
	}
}

func TestRepeatStatSQLKeepsDeferralsOutOfErrors(t *testing.T) {
	normalizedSQL := strings.Join(strings.Fields(repeatStatSQL), " ")
	for _, clause := range []string{
		`COUNT(*) FILTER (WHERE finished_at IS NOT NULL) AS failed_count`,
		`COUNT(*) FILTER (WHERE finished_at IS NULL AND deferred) AS deferred_count`,
		`FILTER (WHERE finished_at IS NOT NULL OR NOT deferred))[1], '') AS last_error`,
		`FILTER (WHERE finished_at IS NULL AND deferred))[1], '') AS last_deferred_reason`,
	} {
		if !strings.Contains(normalizedSQL, clause) {
			t.Fatalf("stat query must contain %q: %s", clause, normalizedSQL)
		}
	}
	// Error classes describe finished failures only; a deferral can never be listed there.
	errorStats := normalizedSQL[strings.Index(normalizedSQL, "error_stats AS"):]
	if !strings.Contains(errorStats, `WHERE finished_at IS NOT NULL GROUP BY topic, "group", error`) {
		t.Fatalf("error classes must be limited to finished repeats: %s", errorStats)
	}
}

func TestUpdateAttemptPersistsDeferred(t *testing.T) {
	client := &execRecorder{}
	ctx := db.AddToContext(context.Background(), client)

	err := (&Repository{}).UpdateAttempt(ctx, &model.Repeat{Id: 7, Attempt: 2, Error: "busy", Deferred: true})
	if err != nil {
		t.Fatalf("update attempt: %v", err)
	}

	normalizedSQL := strings.Join(strings.Fields(client.sql), " ")
	if !strings.Contains(normalizedSQL, `deferred = $5 WHERE id = $6`) {
		t.Fatalf("update must persist the deferred flag: %s", normalizedSQL)
	}
	if client.arguments[4] != true || client.arguments[5] != int64(7) {
		t.Fatalf("unexpected update arguments: %#v", client.arguments)
	}
}

type statRows struct {
	values [][]any
	index  int
}

func (r *statRows) Close()                                         {}
func (r *statRows) Err() error                                     { return nil }
func (r *statRows) CommandTag() pgconn.CommandTag                  { return nil }
func (r *statRows) FieldDescriptions() []pgproto3.FieldDescription { return nil }
func (r *statRows) Values() ([]interface{}, error)                 { return nil, nil }
func (r *statRows) RawValues() [][]byte                            { return nil }

func (r *statRows) Next() bool {
	r.index++
	return r.index < len(r.values)
}

func (r *statRows) Scan(dest ...interface{}) error {
	row := r.values[r.index]
	if len(dest) != len(row) {
		return fmt.Errorf("scan destinations %d, row values %d", len(dest), len(row))
	}
	for i, value := range row {
		switch target := dest[i].(type) {
		case *string:
			*target = value.(string)
		case *int:
			*target = value.(int)
		case **string:
			*target = value.(*string)
		case **time.Time:
			*target = value.(*time.Time)
		default:
			return fmt.Errorf("unexpected scan destination %T", dest[i])
		}
	}
	return nil
}

type statRecorder struct {
	execRecorder
	rows *statRows
}

func (r *statRecorder) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return r.rows, nil
}

func TestGetStatReadsDeferredSeparately(t *testing.T) {
	failedError := "broken"
	failedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	client := &statRecorder{rows: &statRows{index: -1, values: [][]any{
		{"orders", "billing", "broken", "busy, retry after 500 ms", 10, 1, 7, &failedError, 1, &failedAt, &failedAt},
		{"orders", "mailing", "", "busy", 3, 0, 3, (*string)(nil), 0, (*time.Time)(nil), (*time.Time)(nil)},
	}}}
	ctx := db.AddToContext(context.Background(), client)

	stat, err := (&Repository{}).GetStat(ctx)
	if err != nil {
		t.Fatalf("get stat: %v", err)
	}
	if len(stat) != 2 {
		t.Fatalf("unexpected stat: %#v", stat)
	}
	billing := stat[0]
	if billing.AllCount != 10 || billing.FailedCount != 1 || billing.DeferredCount != 7 ||
		billing.LastError != "broken" || billing.LastDeferredReason != "busy, retry after 500 ms" || len(billing.Errors) != 1 {
		t.Fatalf("unexpected billing stat: %#v", billing)
	}
	mailing := stat[1]
	if mailing.FailedCount != 0 || mailing.DeferredCount != 3 || mailing.LastError != "" || len(mailing.Errors) != 0 {
		t.Fatalf("a deferred-only group must report no errors: %#v", mailing)
	}
}

type insertRecorder struct {
	execRecorder
}

type idRow struct{}

func (idRow) Scan(dest ...interface{}) error {
	*dest[0].(*int64) = 1
	return nil
}

func (r *insertRecorder) QueryRow(_ context.Context, sql string, arguments ...interface{}) pgx.Row {
	r.sql = sql
	r.arguments = arguments
	return idRow{}
}

func TestInsertPersistsDeferred(t *testing.T) {
	client := &insertRecorder{}
	ctx := db.AddToContext(context.Background(), client)

	if err := (&Repository{}).Insert(ctx, model.Repeat{Error: "busy", Deferred: true}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	normalizedSQL := strings.Join(strings.Fields(client.sql), " ")
	if !strings.Contains(normalizedSQL, `started_at, deferred) VALUES (`) || !strings.Contains(normalizedSQL, `$12, $13)`) {
		t.Fatalf("insert must persist the deferred flag: %s", normalizedSQL)
	}
	if client.arguments[12] != true {
		t.Fatalf("unexpected insert arguments: %#v", client.arguments)
	}
}
