package model

import (
	"strings"
)

type Message struct {
	Id      string
	Key     []byte
	Value   []byte
	Headers map[string]string
}

type MessageList []Message

func (ml MessageList) GetById(id string) Message {
	m, ok := ml.MessageByID(id)
	if !ok {
		panic("Not found message with id " + id + ", available: " + strings.Join(ml.GetIdList(), ", "))
	}
	return m
}

// MessageByID возвращает сообщение по id Kafka (partition/offset), если оно есть в списке.
func (ml MessageList) MessageByID(id string) (Message, bool) {
	for _, v := range ml {
		if v.Id == id {
			return v, true
		}
	}
	return Message{}, false
}

func (ml MessageList) GetIdList() []string {
	ret := make([]string, 0, len(ml))
	for _, v := range ml {
		ret = append(ret, v.Id)
	}
	return ret
}

func (ml MessageList) IndexByID() map[string]Message {
	ret := make(map[string]Message, len(ml))
	for _, v := range ml {
		ret[v.Id] = v
	}
	return ret
}
