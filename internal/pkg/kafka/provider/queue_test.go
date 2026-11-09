package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/procraft/redbus/internal/app/model"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

type queueKafkaStub struct {
	kafkaClientStub
	requests []*kafka.FetchRequest
	fetch    func(*kafka.FetchRequest) (*kafka.FetchResponse, error)
}

func (s *queueKafkaStub) Fetch(ctx context.Context, request *kafka.FetchRequest) (*kafka.FetchResponse, error) {
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > time.Second {
		panic("unbounded head fetch")
	}
	s.requests = append(s.requests, request)
	return s.fetch(request)
}

func TestQueueSnapshotReadsOnlyPartitionHeadsWithoutPayload(t *testing.T) {
	now := time.Now().Add(-time.Minute).Truncate(time.Second)
	client := &queueKafkaStub{}
	client.fetch = func(request *kafka.FetchRequest) (*kafka.FetchResponse, error) {
		require.Equal(t, int64(headFetchMaxBytes), request.MaxBytes)
		require.Equal(t, 500*time.Millisecond, request.MaxWait)
		require.Equal(t, int64(1), request.MinBytes)
		// Compressed batches may begin before the requested offset. Payload must not be decoded.
		return &kafka.FetchResponse{Records: kafka.NewRecordReader(
			kafka.Record{Offset: request.Offset - 1, Time: now.Add(-time.Hour), Value: &unreadPayload{t: t}},
			kafka.Record{Offset: request.Offset, Time: now.Add(time.Duration(request.Partition) * -time.Minute), Value: &unreadPayload{t: t}},
		)}, nil
	}
	snapshots, err := (&Provider{client: client}).GetQueueSnapshot(context.Background(), model.TopicGroupList{{Topic: "orders", Group: "billing"}})
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.True(t, snapshots[0].Valid)
	require.Equal(t, int64(17), snapshots[0].Lag) // p0 10-8, p1 20-5 (uncommitted starts at retained first).
	require.Equal(t, now.Add(-time.Minute), snapshots[0].Oldest)
	require.Len(t, client.requests, 2)
	require.Equal(t, int64(8), client.requests[0].Offset)
	require.Equal(t, int64(5), client.requests[1].Offset)
}

type unreadPayload struct{ t *testing.T }

func (b *unreadPayload) Read([]byte) (int, error) {
	b.t.Fatal("business payload was read")
	return 0, nil
}
func (b *unreadPayload) Close() error { return nil }

func TestQueueSnapshotEmptyAndUnavailableAreDifferent(t *testing.T) {
	for _, test := range []struct {
		name    string
		offsets *kafka.OffsetFetchResponse
		valid   bool
	}{
		{"empty", &kafka.OffsetFetchResponse{Topics: map[string][]kafka.OffsetFetchPartition{"orders": {{Partition: 0, CommittedOffset: 10}, {Partition: 1, CommittedOffset: 20}}}}, true},
		{"retention-invalid", &kafka.OffsetFetchResponse{Topics: map[string][]kafka.OffsetFetchPartition{"orders": {{Partition: 0, CommittedOffset: 2}, {Partition: 1, CommittedOffset: 20}}}}, false},
		{"fetch-error", &kafka.OffsetFetchResponse{Error: kafka.GroupAuthorizationFailed}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &queueKafkaStub{kafkaClientStub: kafkaClientStub{fetchOverride: test.offsets}}
			client.fetch = func(*kafka.FetchRequest) (*kafka.FetchResponse, error) {
				t.Fatal("no head fetch expected")
				return nil, nil
			}
			values, _ := (&Provider{client: client}).GetQueueSnapshot(context.Background(), model.TopicGroupList{{Topic: "orders", Group: "billing"}})
			require.Len(t, values, 1)
			require.Equal(t, test.valid, values[0].Valid)
			if test.valid {
				require.Zero(t, values[0].Lag)
				require.True(t, values[0].Oldest.IsZero())
			}
			require.Empty(t, client.requests)
		})
	}
}

func TestQueueSnapshotRejectsUnavailableHeadTimestamp(t *testing.T) {
	for _, test := range []struct {
		name     string
		response *kafka.FetchResponse
		err      error
	}{
		{name: "network", err: errors.New("timeout")},
		{name: "broker", response: &kafka.FetchResponse{Error: kafka.OffsetOutOfRange}},
		{name: "no-records", response: &kafka.FetchResponse{Records: kafka.NewRecordReader()}},
		{name: "no-timestamp", response: &kafka.FetchResponse{Records: kafka.NewRecordReader(kafka.Record{Offset: 8})}},
		{name: "future-timestamp", response: &kafka.FetchResponse{Records: kafka.NewRecordReader(kafka.Record{Offset: 8, Time: time.Now().Add(time.Hour)})}},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &queueKafkaStub{}
			client.fetch = func(*kafka.FetchRequest) (*kafka.FetchResponse, error) { return test.response, test.err }
			_, err := (&Provider{client: client}).headTimestamp(context.Background(), "orders", 0, 8)
			require.Error(t, err)
		})
	}
}

func TestQueueSnapshotBoundsRecordInspection(t *testing.T) {
	records := make([]kafka.Record, headFetchMaxRecords+1)
	for i := range records {
		records[i] = kafka.Record{Offset: int64(i), Time: time.Now()}
	}
	client := &queueKafkaStub{fetch: func(*kafka.FetchRequest) (*kafka.FetchResponse, error) {
		return &kafka.FetchResponse{Records: kafka.NewRecordReader(records...)}, nil
	}}
	_, err := (&Provider{client: client}).headTimestamp(context.Background(), "orders", 0, model.Offset(headFetchMaxRecords))
	require.ErrorContains(t, err, "bounded")
}

func (b *unreadPayload) Len() int { return 0 }

func TestQueueHeadEpochTimestampIsUnavailable(t *testing.T) {
	client := &queueKafkaStub{fetch: func(*kafka.FetchRequest) (*kafka.FetchResponse, error) {
		return &kafka.FetchResponse{Records: kafka.NewRecordReader(kafka.Record{Offset: 8, Time: time.Unix(0, 0)})}, nil
	}}
	_, err := (&Provider{client: client}).headTimestamp(context.Background(), "orders", 0, 8)
	require.ErrorContains(t, err, "timestamp")
}
