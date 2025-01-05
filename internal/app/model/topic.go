package model

type Topic struct {
	Name          string      `json:"name"`
	PartitionList []Partition `json:"partition_list"`
	Group         string      `json:"group"`
	Offset        int         `json:"offset"`
	LastOffset    int         `json:"last_offset"`
	ConsumerCount int         `json:"consumer_count"`
}

type Partition struct {
	N           int   `json:"n"`
	FirstOffset int64 `json:"first_offset"`
	LastOffset  int64 `json:"last_offset"`
}

type TopicList = []Topic
