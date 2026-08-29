package model

import "encoding/json"

type Event interface {
	GetName() string
	GetData() string
}

type EventConsumers struct {
	ConsumerCount     int `json:"consumerCount"`
	ConsumeTopicCount int `json:"consumeTopicCount"`
}

func (e EventConsumers) GetName() string { return "consumers" }
func (e EventConsumers) GetData() string {
	data, _ := json.Marshal(e)
	return string(data)
}

type EventRepeater struct {
	AllCount    int `json:"allCount"`
	FailedCount int `json:"failedCount"`
}

func (e EventRepeater) GetName() string { return "repeater" }
func (e EventRepeater) GetData() string {
	data, _ := json.Marshal(e)
	return string(data)
}
