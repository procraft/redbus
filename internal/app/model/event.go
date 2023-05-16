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

type EventRepeater struct {
	AllCount   int
	Failedount int
}

func (e EventRepeater) GetName() string { return "repeater" }
func (e EventRepeater) GetData() string {
	return `{"allCount": ` + strconv.Itoa(e.AllCount) + `, "failedount": ` + strconv.Itoa(e.Failedount) + "}"
}
