package model

import (
	"context"
	"time"
)

type TopicName string
type GroupName string
type ConsumerId string
type PartitionN int
type Offset int64

type PartitionOffsetMap = map[PartitionN]Offset

type ConsumerMetrics struct {
	ConnectedAt       time.Time
	StateSince        time.Time
	LastMessageAt     time.Time
	MessagesProcessed uint64
	ReconnectCount    uint64
	LastError         string
}

type ConsumerState int32

func (s ConsumerState) String() string {
	switch s {
	case ConsumerStateConnecting:
		return "connecting"
	case ConsumerStateConnected:
		return "connected"
	case ConsumerStateReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

const (
	ConsumerStateConnecting   ConsumerState = 1
	ConsumerStateConnected    ConsumerState = 2
	ConsumerStateReconnecting ConsumerState = 3
)

type IConsumer interface {
	GetHosts() []string
	GetTopic() TopicName
	GetGroup() GroupName
	GetID() ConsumerId
	GetState() ConsumerState
	SetState(state ConsumerState)
	GetOffsetMap() PartitionOffsetMap
	GetMetrics() ConsumerMetrics
	Consume(ctx context.Context, processor func(ctx context.Context, list MessageList) error) error
	Lock()
	Unlock()
	Close() (bool, error)
	Reconnect(ctx context.Context) error
}
