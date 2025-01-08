package connstore

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
)

type ConsumerBag struct {
	Consumer       model.IConsumer
	Server         pb.RedbusService_ConsumeServer
	RepeatStrategy *model.RepeatStrategy
}

type ConsumerStore struct {
	store  map[ConsumerKey]ConsumerBag
	random *rand.Rand
	mu     sync.RWMutex
}

type ConsumerKey struct {
	Topic model.TopicName
	Group model.GroupName
	Id    model.ConsumerId
}

func (k ConsumerKey) String() string {
	return fmt.Sprintf("%s!%s", k.Topic, k.Group)
}

func NewConsumerStore() *ConsumerStore {
	randomSource := rand.NewSource(time.Now().Unix())
	random := rand.New(randomSource)
	return &ConsumerStore{
		store:  make(map[ConsumerKey]ConsumerBag),
		random: random,
	}
}

func (s *ConsumerStore) add(c model.IConsumer, repeatStrategy *model.RepeatStrategy, srv pb.RedbusService_ConsumeServer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[s.getKey(c)] = ConsumerBag{Consumer: c, Server: srv, RepeatStrategy: repeatStrategy}
}

func (s *ConsumerStore) remove(c model.IConsumer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, s.getKey(c))
}

func (s *ConsumerStore) getTopicGroupList() model.TopicGroupList {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ret := make(model.TopicGroupList, 0, len(s.store))
	exists := make(map[string]struct{}, len(s.store))
	for k := range s.store {
		key := k.String()
		if _, ok := exists[key]; !ok {
			exists[key] = struct{}{}
			ret = append(ret, model.TopicGroup{Topic: k.Topic, Group: k.Group})
		}
	}
	return ret
}

func (s *ConsumerStore) getOffsetMap() map[ConsumerKey]model.PartitionOffsetMap {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ret := make(map[ConsumerKey]model.PartitionOffsetMap, len(s.store))
	for k, v := range s.store {
		ret[k] = v.Consumer.GetOffsetMap()
	}
	return ret
}

func (s *ConsumerStore) getKey(c model.IConsumer) ConsumerKey {
	return ConsumerKey{Topic: c.GetTopic(), Group: c.GetGroup(), Id: c.GetID()}
}

func (s *ConsumerStore) getState(consumerId model.ConsumerId) model.ConsumerState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, v := range s.store {
		if k.Id == consumerId {
			return v.Consumer.GetState()
		}
	}
	return 0
}

func (s *ConsumerStore) findBest(topic model.TopicName, group model.GroupName, id model.ConsumerId) *ConsumerBag {
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
