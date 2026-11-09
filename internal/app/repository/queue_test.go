package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/procraft/redbus/internal/app/model"
	"github.com/procraft/redbus/internal/pkg/db"
	"github.com/stretchr/testify/require"
)

type queueRows struct {
	stringRows
	stats                 []model.RetryQueueStat
	scanError, errorAtEnd error
	closed                bool
}

func (r *queueRows) Close()     { r.closed = true }
func (r *queueRows) Err() error { return r.errorAtEnd }
func (r *queueRows) Next() bool { r.index++; return r.index < len(r.stats) }
func (r *queueRows) Scan(dest ...interface{}) error {
	if r.scanError != nil {
		return r.scanError
	}
	stat := r.stats[r.index]
	*dest[0].(*model.TopicName) = stat.Topic
	*dest[1].(*model.GroupName) = stat.Group
	*dest[2].(*int) = stat.All
	*dest[3].(*int) = stat.Failed
	*dest[4].(*int) = stat.Deferred
	*dest[5].(**time.Time) = stat.Oldest
	return nil
}

type queueDB struct {
	execRecorder
	rows  *queueRows
	query string
	err   error
	calls int
}

func (d *queueDB) Query(_ context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	d.query = sql
	d.calls++
	return d.rows, d.err
}

func TestRetryQueueAggregationReturnsCountsAndNullableOldestInOneQuery(t *testing.T) {
	oldest := time.Now().Add(-time.Hour)
	stats := []model.RetryQueueStat{{Topic: "email", Group: "operational", All: 10, Failed: 2, Deferred: 3, Oldest: &oldest}, {Topic: "email", Group: "auth", All: 2, Failed: 2}}
	rows := &queueRows{stringRows: stringRows{index: -1}, stats: stats}
	database := &queueDB{rows: rows}
	result, err := New().GetQueueStats(db.AddToContext(context.Background(), database))
	require.NoError(t, err)
	require.Equal(t, stats, result)
	require.Equal(t, 1, database.calls)
	require.True(t, rows.closed)
	query := strings.Join(strings.Fields(database.query), " ")
	require.Contains(t, query, `MIN(created_at) FILTER (WHERE finished_at IS NULL)`)
	require.Contains(t, query, `GROUP BY topic, "group"`)
	require.NotContains(t, query, "headers")
	require.NotContains(t, query, "data")
}

func TestRetryQueueAggregationDoesNotPublishPartialRowsOnError(t *testing.T) {
	for _, test := range []struct {
		name                            string
		queryError, scanError, endError error
	}{
		{name: "query", queryError: errors.New("timeout")},
		{name: "scan", scanError: errors.New("invalid timestamp")},
		{name: "iteration", endError: errors.New("cancelled")},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := &queueRows{stringRows: stringRows{index: -1}, stats: []model.RetryQueueStat{{Topic: "email"}}, scanError: test.scanError, errorAtEnd: test.endError}
			_, err := New().GetQueueStats(db.AddToContext(context.Background(), &queueDB{rows: rows, err: test.queryError}))
			require.Error(t, err)
		})
	}
}
