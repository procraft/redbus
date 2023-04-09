package model

import (
	"math/rand"
	"time"
)

type ConsumerBag struct {
	consumer       IConsumer
	repeatStrategy *RepeatStrategy
}

type ConsumerStore struct {
	store  map[ConsumerKey]ConsumerBag
	random *rand.Rand
}

type ConsumerKey struct {
	Topic string
	Group string
	Id    string
}

type IConsumer interface {
}

func NewConsumerStore() *ConsumerStore {
	randomSource := rand.NewSource(time.Now().Unix())
	random := rand.New(randomSource)
	return &ConsumerStore{
		store:  make(map[ConsumerKey]ConsumerBag),
		random: random,
	}
}

func (s *ConsumerStore) Add(topic, group, id string, repeatStrategy *RepeatStrategy, c IConsumer) {
	s.store[s.getKey(topic, group, id)] = ConsumerBag{consumer: c, repeatStrategy: repeatStrategy}
}

func (s *ConsumerStore) Remove(topic, group, id string) {
	delete(s.store, s.getKey(topic, group, id))
}

func (s *ConsumerStore) GetTopicGroupList() TopicGroupList {
	ret := make(TopicGroupList, 0, len(s.store))
	exists := make(map[string]struct{}, len(s.store))
	for k := range s.store {
		key := k.Topic + "!" + k.Group
		if _, ok := exists[key]; !ok {
			exists[key] = struct{}{}
			ret = append(ret, TopicGroup{Topic: k.Topic, Group: k.Group})
		}
	}
	return ret
}

func (s *ConsumerStore) FindStrategy(topic, group, id string) *RepeatStrategy {
	c := s.findBest(topic, group, id)
	if c == nil {
		return nil
	}
	return c.repeatStrategy
}

func (s *ConsumerStore) getKey(topic, group, id string) ConsumerKey {
	return ConsumerKey{Topic: topic, Group: group, Id: id}
}

func (s *ConsumerStore) findBest(topic, group, id string) *ConsumerBag {
	list := make([]ConsumerBag, 0, len(s.store))
	for k, v := range s.store {
		if k.Topic == topic && k.Group == group && k.Id == id {
			return &v
		}
		list = append(list)
	}
	if len(list) != 0 {
		bag := list[s.random.Intn(len(list))]
		return &bag
	}
	return nil
}
