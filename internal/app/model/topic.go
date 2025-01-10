package model

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
	PartitionList []StatGroupPartition `json:"partitions"`
}

type StatGroupPartition struct {
	N             PartitionN `json:"n"`
	Offset        Offset     `json:"offset"`
	ConsumerId    ConsumerId `json:"consumerId"`
	ConsumerState string     `json:"consumerState"`
}

type StatTopicList = []StatTopic
