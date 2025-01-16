package model

type RepeatData struct {
	Topic      TopicName
	Group      GroupName
	ConsumerId ConsumerId
	MessageId  string
	Key        *[]byte
	Message    []byte
	Headers    map[string]string
	Strategy   *RepeatStrategy
}
