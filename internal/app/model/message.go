package model

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
	panic("Not found message with id " + id)
}
