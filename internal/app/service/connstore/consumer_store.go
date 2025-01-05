package connstore

import (
	"math/rand"
	"sync"
	"time"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
)

type ConsumerBag struct {
	Consumer       model.IConsumer
	Srv            pb.RedbusService_ConsumeServer
	RepeatStrategy *model.RepeatStrategy
}

type ConsumerStore struct {
	store  map[ConsumerKey]ConsumerBag
	random *rand.Rand
	mu     sync.RWMutex
}

type ConsumerKey struct {
	Topic string
	Group string
	Id    string
}

func NewConsumerStore() *ConsumerStore {
	randomSource := rand.NewSource(time.Now().Unix())
	random := rand.New(randomSource)
	return &ConsumerStore{
		store:  make(map[ConsumerKey]ConsumerBag),
		random: random,
	}
}

func (s *ConsumerStore) add(topic, group, id string, repeatStrategy *model.RepeatStrategy, c model.IConsumer, srv pb.RedbusService_ConsumeServer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[s.getKey(topic, group, id)] = ConsumerBag{Consumer: c, Srv: srv, RepeatStrategy: repeatStrategy}
}

func (s *ConsumerStore) remove(topic, group, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, s.getKey(topic, group, id))
}

func (s *ConsumerStore) getTopicGroupList() model.TopicGroupList {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ret := make(model.TopicGroupList, 0, len(s.store))
	exists := make(map[string]struct{}, len(s.store))
	for k := range s.store {
		key := k.Topic + "!" + k.Group
		if _, ok := exists[key]; !ok {
			exists[key] = struct{}{}
			ret = append(ret, model.TopicGroup{Topic: k.Topic, Group: k.Group})
		}
	}
	return ret
}

func (s *ConsumerStore) getKey(topic, group, id string) ConsumerKey {
	return ConsumerKey{Topic: topic, Group: group, Id: id}
}

func (s *ConsumerStore) findBest(topic, group, id string) *ConsumerBag {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]ConsumerBag, 0, len(s.store))
	if bag, ok := s.store[ConsumerKey{Topic: topic, Group: group, Id: id}]; ok {
		return &bag
	}
	for k, v := range s.store {
		if k.Topic == topic && k.Group == group {
			list = append(list, v)
		}
	}
	if len(list) != 0 {
		bag := list[s.random.Intn(len(list))]
		return &bag
	}
	return nil
}
