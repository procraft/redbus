package model

import (
	"encoding/json"
	"testing"
)

func TestEventConsumersGetData(t *testing.T) {
	event := EventConsumers{ConsumerCount: 3, ConsumeTopicCount: 5}

	var data struct {
		ConsumerCount     int `json:"consumerCount"`
		ConsumeTopicCount int `json:"consumeTopicCount"`
	}
	if err := json.Unmarshal([]byte(event.GetData()), &data); err != nil {
		t.Fatalf("unmarshal event data: %v", err)
	}
	if data.ConsumerCount != event.ConsumerCount || data.ConsumeTopicCount != event.ConsumeTopicCount {
		t.Fatalf("unexpected event data: %+v", data)
	}
}

func TestEventRepeaterGetData(t *testing.T) {
	event := EventRepeater{AllCount: 8, FailedCount: 2}

	var data struct {
		AllCount    int `json:"allCount"`
		FailedCount int `json:"failedCount"`
	}
	if err := json.Unmarshal([]byte(event.GetData()), &data); err != nil {
		t.Fatalf("unmarshal event data: %v", err)
	}
	if data.AllCount != event.AllCount || data.FailedCount != event.FailedCount {
		t.Fatalf("unexpected event data: %+v", data)
	}
}
