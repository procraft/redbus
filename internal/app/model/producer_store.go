package model

import "context"

type ProducerStore struct {
	createFn CreateProducerFn
	store    map[string]IProducer
}

type IProducer interface {
	Produce(ctx context.Context, keyAndMessage ...[]byte) error
	Close() error
}

type CreateProducerFn = func(ctx context.Context, topic string) (IProducer, error)

func NewProducerStore(createFn CreateProducerFn) *ProducerStore {
	return &ProducerStore{
		createFn: createFn,
		store:    make(map[string]IProducer),
	}
}

func (s *ProducerStore) Get(ctx context.Context, topic string) (IProducer, error) {
	if p := s.store[topic]; p != nil {
		return p, nil
	}
	p, err := s.createFn(ctx, topic)
	if err != nil {
		return nil, err
	}
	s.store[topic] = p
	return p, nil
}
