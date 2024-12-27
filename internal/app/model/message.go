package model

import "strings"

type Message struct {
	Id    string
	Key   []byte
	Value []byte
}

type MessageList []Message

func (ml MessageList) GetById(id string) Message {
	for _, v := range ml {
		if v.Id == id {
			return v
		}
	}
	panic("Not found message with id " + id + ", available: " + strings.Join(ml.getIdList(), ", "))
}

func (ml MessageList) getIdList() []string {
	ret := make([]string, 0, len(ml))
	for _, v := range ml {
		ret = append(ret, v.Id)
	}
	return ret
}
