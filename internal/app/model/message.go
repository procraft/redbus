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
	for _, v := range ml {
		if v.Id == id {
			return v
		}
	}
	panic("Not found message with id " + id + ", available: " + strings.Join(ml.GetIdList(), ", "))
}

func (ml MessageList) GetIdList() []string {
	ret := make([]string, 0, len(ml))
	for _, v := range ml {
		ret = append(ret, v.Id)
	}
	return ret
}
