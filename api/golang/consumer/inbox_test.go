package consumer

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/procraft/redbus/api/golang/inbox"
)

type fakeInboxStore struct {
	processed map[string]bool
	marked    []string
	checkErr  error
	markErr   error
}

func (s *fakeInboxStore) isProcessed(_ context.Context, group, topic, key string) (bool, error) {
	return s.processed[inbox.Key(group, topic, key)], s.checkErr
}

func (s *fakeInboxStore) setProcessed(_ context.Context, group, topic, key string) error {
	s.marked = append(s.marked, inbox.Key(group, topic, key))
	return s.markErr
}

type handlerCall struct {
	calls int
	msg   Message
}

func (h *handlerCall) handler(err error) Handler {
	return func(_ context.Context, msg Message) error {
		h.calls++
		h.msg = msg
		return err
	}
}

func process(store inboxStore, mode inbox.Mode, msg Message, h Handler) error {
	return processWithInbox(context.Background(), store, mode, "g", "t", msg, h, slog.Default())
}

func TestInboxDisabledAlwaysRunsHandler(t *testing.T) {
	store := &fakeInboxStore{processed: map[string]bool{"g|t|k": true}}
	var h handlerCall
	require.NoError(t, process(store, inbox.Disabled, Message{ID: "id", IdempotencyKey: "k"}, h.handler(nil)))
	require.Equal(t, 1, h.calls)
	require.Nil(t, h.msg.Claim)
	require.Empty(t, store.marked)

	require.NoError(t, process(nil, inbox.OnlyOnce, Message{ID: "id"}, h.handler(nil)))
	require.Equal(t, 2, h.calls)
}

func TestInboxOnlyOnceMarksAfterSuccess(t *testing.T) {
	store := &fakeInboxStore{}
	var h handlerCall
	require.NoError(t, process(store, inbox.OnlyOnce, Message{ID: "id", IdempotencyKey: "k"}, h.handler(nil)))
	require.Equal(t, 1, h.calls)
	require.Nil(t, h.msg.Claim)
	require.Equal(t, []string{"g|t|k"}, store.marked)
}

func TestInboxOnlyOnceDoesNotMarkFailure(t *testing.T) {
	store := &fakeInboxStore{}
	var h handlerCall
	require.EqualError(t, process(store, inbox.OnlyOnce, Message{ID: "id", IdempotencyKey: "k"}, h.handler(errors.New("boom"))), "boom")
	require.Empty(t, store.marked)
}

func TestInboxOnlyOnceKeepsSuccessWhenMarkFails(t *testing.T) {
	store := &fakeInboxStore{markErr: errors.New("db down")}
	var h handlerCall
	require.NoError(t, process(store, inbox.OnlyOnce, Message{ID: "id", IdempotencyKey: "k"}, h.handler(nil)))
	require.Equal(t, 1, h.calls)
}

func TestInboxSkipsProcessedMessage(t *testing.T) {
	for _, mode := range []inbox.Mode{inbox.OnlyOnce, inbox.Transactional} {
		store := &fakeInboxStore{processed: map[string]bool{"g|t|k": true}}
		var h handlerCall
		require.NoError(t, process(store, mode, Message{ID: "id", IdempotencyKey: "k"}, h.handler(nil)), mode.String())
		require.Zero(t, h.calls, mode.String())
		require.Empty(t, store.marked, mode.String())
	}
}

func TestInboxFallsBackToMessageID(t *testing.T) {
	store := &fakeInboxStore{}
	var h handlerCall
	require.NoError(t, process(store, inbox.OnlyOnce, Message{ID: "id"}, h.handler(nil)))
	require.Equal(t, []string{"g|t|id"}, store.marked)
}

func TestInboxTransactionalHandsClaimAndNeverMarks(t *testing.T) {
	store := &fakeInboxStore{}
	var h handlerCall
	require.NoError(t, process(store, inbox.Transactional, Message{ID: "id", IdempotencyKey: "k"}, h.handler(nil)))
	require.Equal(t, 1, h.calls)
	require.NotNil(t, h.msg.Claim)
	require.Empty(t, store.marked)
}

func TestInboxCheckErrorFailsMessage(t *testing.T) {
	store := &fakeInboxStore{checkErr: errors.New("db down")}
	var h handlerCall
	require.EqualError(t, process(store, inbox.OnlyOnce, Message{ID: "id"}, h.handler(nil)), "db down")
	require.Zero(t, h.calls)
}
