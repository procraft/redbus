package connstore

import (
	"context"
	"testing"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/stretchr/testify/require"
)

type consumerStub struct {
	topic   model.TopicName
	group   model.GroupName
	id      model.ConsumerId
	state   model.ConsumerState
	offsets model.PartitionOffsetMap
}

func (c *consumerStub) GetHosts() []string                     { return nil }
func (c *consumerStub) GetTopic() model.TopicName              { return c.topic }
func (c *consumerStub) GetGroup() model.GroupName              { return c.group }
func (c *consumerStub) GetID() model.ConsumerId                { return c.id }
func (c *consumerStub) GetState() model.ConsumerState          { return c.state }
func (c *consumerStub) SetState(state model.ConsumerState)     { c.state = state }
func (c *consumerStub) GetOffsetMap() model.PartitionOffsetMap { return c.offsets }
func (c *consumerStub) GetMetrics() model.ConsumerMetrics      { return model.ConsumerMetrics{} }
func (c *consumerStub) Consume(context.Context, func(context.Context, model.MessageList) error) error {
	return nil
}
func (c *consumerStub) Lock()                           {}
func (c *consumerStub) Unlock()                         {}
func (c *consumerStub) Close() (bool, error)            { return true, nil }
func (c *consumerStub) Reconnect(context.Context) error { return nil }

func TestGetStatTopicGroupPartitionIncludesConsumersWithoutOffsets(t *testing.T) {
	store := New(nil)
	store.AddConsumer(&consumerStub{
		topic: "orders", group: "billing", id: "worker-1", state: model.ConsumerStateConnecting,
		offsets: model.PartitionOffsetMap{},
	}, nil, nil)
	store.AddConsumer(&consumerStub{
		topic: "orders", group: "billing", id: "worker-2", state: model.ConsumerStateConnected,
		offsets: model.PartitionOffsetMap{0: 15},
	}, nil, nil)

	groups := store.GetStatTopicGroupPartition()["orders"]
	require.Len(t, groups, 1)
	require.Equal(t, model.GroupName("billing"), groups[0].Name)
	require.Len(t, groups[0].ConsumerList, 2)
	consumers := make(map[model.ConsumerId]model.StatConsumer)
	for _, consumer := range groups[0].ConsumerList {
		consumers[consumer.Id] = consumer
	}
	require.Equal(t, "connecting", consumers["worker-1"].State)
	require.Equal(t, model.TopicName("orders"), consumers["worker-1"].Topic)
	require.Equal(t, model.GroupName("billing"), consumers["worker-1"].Group)
	require.Empty(t, consumers["worker-1"].PartitionList)
	require.Equal(t, "connected", consumers["worker-2"].State)
	require.Equal(t, []model.StatConsumerPartition{{
		N: 0, GroupOffset: 15, Committed: true,
	}}, consumers["worker-2"].PartitionList)
	require.Equal(t, []model.StatGroupPartition{{
		N: 0, Offset: 15, Committed: true, ConsumerId: "worker-2", ConsumerState: "connected",
	}}, groups[0].PartitionList)
}
