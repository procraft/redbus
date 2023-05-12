package model

import "strconv"

type Event interface {
	GetName() string
	GetData() string
}

type EventConsumers struct {
	ConsumerCount     int
	ConsumeTopicCount int
}

func (e EventConsumers) GetName() string { return "consumers" }
func (e EventConsumers) GetData() string {
	return `{"consumerCount": ` + strconv.Itoa(e.ConsumerCount) + `, "consumeTopicCount": ` + strconv.Itoa(e.ConsumeTopicCount) + "}"
}
