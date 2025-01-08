package model

type RepeatData struct {
	Topic      TopicName
	Group      GroupName
	ConsumerId ConsumerId
	MessageId  string
	Key        *[]byte
	Message    []byte
	Strategy   *RepeatStrategy
}
