package model

import "time"

type StatTopic struct {
	Name          TopicName       `json:"name"`
	PartitionList []StatPartition `json:"partitions"`
	GroupList     []StatGroup     `json:"groups"`
}

type StatPartition struct {
	N           PartitionN `json:"n"`
	FirstOffset Offset     `json:"firstOffset"`
	LastOffset  Offset     `json:"lastOffset"`
}

type StatGroup struct {
	Name          GroupName            `json:"name"`
	KafkaGroupId  string               `json:"kafkaGroupId"`
	State         string               `json:"state"`
	Error         string               `json:"error"`
	PartitionList []StatGroupPartition `json:"partitions"`
	ConsumerList  []StatConsumer       `json:"consumers"`
}

type StatConsumer struct {
	Id                ConsumerId              `json:"id"`
	Topic             TopicName               `json:"topic"`
	Group             GroupName               `json:"group"`
	State             string                  `json:"state"`
	ConnectedAt       time.Time               `json:"connectedAt"`
	StateSince        time.Time               `json:"stateSince"`
	LastMessageAt     *time.Time              `json:"lastMessageAt"`
	MessagesProcessed uint64                  `json:"messagesProcessed"`
	ReconnectCount    uint64                  `json:"reconnectCount"`
	LastError         string                  `json:"lastError"`
	RepeatStrategy    string                  `json:"repeatStrategy"`
	KafkaMemberId     string                  `json:"kafkaMemberId"`
	ClientHost        string                  `json:"clientHost"`
	PartitionList     []StatConsumerPartition `json:"partitions"`
}

type StatConsumerPartition struct {
	N           PartitionN `json:"n"`
	GroupOffset Offset     `json:"groupOffset"`
	LastOffset  Offset     `json:"lastOffset"`
	Lag         Offset     `json:"lag"`
	Committed   bool       `json:"committed"`
}

type StatGroupPartition struct {
	N             PartitionN `json:"n"`
	Offset        Offset     `json:"offset"`
	FirstOffset   Offset     `json:"firstOffset"`
	LastOffset    Offset     `json:"lastOffset"`
	Lag           Offset     `json:"lag"`
	Committed     bool       `json:"committed"`
	ConsumerId    ConsumerId `json:"consumerId"`
	ConsumerState string     `json:"consumerState"`
}

type StatTopicList = []StatTopic
type StatConsumerList = []StatConsumer
