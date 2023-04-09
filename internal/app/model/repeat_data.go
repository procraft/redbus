package model

type RepeatData struct {
	Topic      string
	Group      string
	ConsumerId string
	MessageId  string
	Key        []byte
	Message    []byte
	Strategy   *RepeatStrategy
}
