package model

import "time"

// QueueSnapshot is a background observation; zero Oldest means a valid empty queue.
type QueueSnapshot struct {
	Topic  TopicName
	Group  GroupName
	Lag    int64
	Oldest time.Time
	Valid  bool
}

// RetryQueueStat shares the existing metrics count scan; Oldest starts at the first failure.
type RetryQueueStat struct {
	Topic    TopicName
	Group    GroupName
	All      int
	Failed   int
	Deferred int
	Oldest   *time.Time
}
