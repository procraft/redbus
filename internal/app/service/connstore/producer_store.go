package connstore

import (
	"context"
	"sync"

	"github.com/prokraft/redbus/internal/app/model"
)

type ProducerStore struct {
	createFn CreateProducerFn
	store    map[string]model.IProducer
	mu       sync.Mutex
}

type CreateProducerFn = func(ctx context.Context, topic string) (model.IProducer, error)

func NewProducerStore(createFn CreateProducerFn) *ProducerStore {
	return &ProducerStore{
		createFn: createFn,
		store:    make(map[string]model.IProducer),
	}
}

func (s *ProducerStore) get(ctx context.Context, topic string) (model.IProducer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
