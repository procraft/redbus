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

type consumerStatSnapshot struct {
	offsetMap      model.PartitionOffsetMap
	state          model.ConsumerState
	metrics        model.ConsumerMetrics
	repeatStrategy *model.RepeatStrategy
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

func (s *ConsumerStore) count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.store)
}

func (s *ConsumerStore) consumeTopicCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	topics := make(map[model.TopicName]struct{}, len(s.store))
	for consumer := range s.store {
		topics[consumer.Topic] = struct{}{}
	}
	return len(topics)
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

func (s *ConsumerStore) getStatSnapshot() map[ConsumerKey]consumerStatSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ret := make(map[ConsumerKey]consumerStatSnapshot, len(s.store))
	for k, v := range s.store {
		ret[k] = consumerStatSnapshot{
			offsetMap:      v.Consumer.GetOffsetMap(),
			state:          v.Consumer.GetState(),
			metrics:        v.Consumer.GetMetrics(),
			repeatStrategy: v.RepeatStrategy,
		}
	}
	return ret
}

func (s *ConsumerStore) getKey(c model.IConsumer) ConsumerKey {
	return ConsumerKey{Topic: c.GetTopic(), Group: c.GetGroup(), Id: c.GetID()}
}

func (s *ConsumerStore) findBest(topic model.TopicName, group model.GroupName, id model.ConsumerId) *ConsumerBag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := ConsumerKey{Topic: topic, Group: group, Id: id}
	if bag, ok := s.store[key]; ok {
		return &bag
	}

	var candidates []ConsumerBag
	for k, v := range s.store {
		if k.Topic == topic && k.Group == group {
			candidates = append(candidates, v)
		}
	}

	if len(candidates) > 0 {
		selected := candidates[s.random.Intn(len(candidates))]
		return &selected
	}
	return nil
}
