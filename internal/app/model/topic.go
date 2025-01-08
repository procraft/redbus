package model

type StatTopic struct {
	Name          TopicName       `json:"name"`
	PartitionList []StatPartition `json:"partitions"`
	GroupList     []StatGroup     `json:"groups"`
}

type StatPartition struct {
	N           PartitionN `json:"n"`
	FirstOffset Offset     `json:"first_offset"`
	LastOffset  Offset     `json:"last_offset"`
}

type StatGroup struct {
	Name          GroupName            `json:"name"`
	PartitionList []StatGroupPartition `json:"partitions"`
}

type StatGroupPartition struct {
	N             PartitionN `json:"n"`
	Offset        Offset     `json:"offset"`
	ConsumerId    ConsumerId `json:"consumer_id"`
	ConsumerState string     `json:"consumer_state"`
}

type StatTopicList = []StatTopic
