package outbox

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/procraft/redbus/api/golang/producer"
)

type recordingExecer struct{ execs int }

func (r *recordingExecer) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	r.execs++
	return nil, nil
}

func TestWriteChecksTheDefaultLimitBeforeTheInsert(t *testing.T) {
	tx := &recordingExecer{}
	ctx := context.Background()

	require.NoError(t, Write(ctx, tx, "t", make([]byte, producer.DefaultMaxMessageBytes)))
	require.ErrorIs(t, Write(ctx, tx, "t", make([]byte, producer.DefaultMaxMessageBytes+1)), producer.ErrMessageTooLarge)
	require.Equal(t, 1, tx.execs)
}

func TestWriteWithLimitAppliesTheGivenLimit(t *testing.T) {
	tx := &recordingExecer{}
	ctx := context.Background()

	require.NoError(t, WriteWithLimit(ctx, tx, "t", make([]byte, 10), 10))
	err := WriteWithLimit(ctx, tx, "t", make([]byte, 11), 10)
	require.Equal(t, &producer.MessageTooLargeError{Topic: "t", Size: 11, Limit: 10}, err)
	require.Equal(t, 1, tx.execs)
}
