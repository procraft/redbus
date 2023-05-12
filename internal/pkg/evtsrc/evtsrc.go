package evtsrc

import "github.com/sergiusd/redbus/internal/app/model"

type EventSource struct {
	handler func(event model.Event)
}

func New() *EventSource {
	return &EventSource{
		handler: func(event model.Event) {},
	}
}

func (ep *EventSource) Handler(handler func(event model.Event)) {
	ep.handler = handler
}

func (ep *EventSource) Publish(fn func() model.Event) {
	go ep.handler(fn())
}
