package repeater

import (
	"context"
	"sync"

	"github.com/sergiusd/redbus/internal/app/grpcapi"
	"github.com/sergiusd/redbus/internal/app/model"
	"github.com/sergiusd/redbus/internal/app/service/connstore"
	"github.com/sergiusd/redbus/internal/pkg/logger"

	"github.com/sergiusd/redbus/internal/pkg/runtime"
)

type Repeater struct {
	defaultStrategy *model.RepeatStrategy
	connStore       IConnStore
	repo            IRepository
}

type IRepository interface {
	Insert(ctx context.Context, repeat model.Repeat) error
	FindForRepeat(ctx context.Context, topicGroupList model.TopicGroupList) (model.RepeatList, error)
	Delete(ctx context.Context, repeatId int64) error
	UpdateAttempt(ctx context.Context, repeat *model.Repeat) error
	GetCount(ctx context.Context) (int, int, error)
}

type IConnStore interface {
	GetConsumerTopicGroupList() model.TopicGroupList
	FindBestConsumerBag(topic, group, id string) *connstore.ConsumerBag
}

func New(defaultStrategy *model.RepeatStrategy, connStore IConnStore, repo IRepository) *Repeater {
	return &Repeater{
		defaultStrategy: defaultStrategy,
		connStore:       connStore,
		repo:            repo,
	}
}

func (r *Repeater) Add(ctx context.Context, data model.RepeatData, errorMsg string) error {
	repeat := model.Repeat{
		Topic:      data.Topic,
		Group:      data.Group,
		ConsumerId: data.ConsumerId,
		MessageId:  data.MessageId,
		Error:      errorMsg,
		Key:        data.Key,
		Data:       data.Message,
		Strategy:   data.Strategy,
		CreatedAt:  runtime.Now(),
	}
	repeat.SetZeroAttempt(r.defaultStrategy)
	return r.repo.Insert(ctx, repeat)
}

func (r *Repeater) Repeat(ctx context.Context) error {
	topicGroupList := r.connStore.GetConsumerTopicGroupList()
	repeatList, err := r.repo.FindForRepeat(ctx, topicGroupList)
	if err != nil {
		return err
	}
	if len(repeatList) == 0 {
		return nil
	}
	groupedRepeat := repeatList.GroupByConsumerId()
	logger.Info(ctx, "Start repeater iteration: %d messages, %d consumers", len(repeatList), len(groupedRepeat))
	var wg sync.WaitGroup
	wg.Add(len(groupedRepeat))
	for _, consumerRepeatList := range groupedRepeat {
		go func(list model.RepeatList) {
			r.repeatConsumer(ctx, list)
			wg.Done()
		}(consumerRepeatList)
	}
	wg.Wait()
	return nil
}

func (r *Repeater) GetCount(ctx context.Context) (int, int, error) {
	return r.repo.GetCount(ctx)
}

func (r *Repeater) repeatConsumer(ctx context.Context, repeatList model.RepeatList) {
	for _, repeat := range repeatList {
		bag := r.connStore.FindBestConsumerBag(repeat.Topic, repeat.Group, repeat.ConsumerId)
		if bag == nil {
			continue
		}
		data, err := grpcapi.SendToConsumerAndWaitResponse(logger.App, bag.Consumer, bag.Srv, model.MessageList{{Id: repeat.MessageId, Value: repeat.Data}})
		if err != nil {
			logger.Error(ctx, "Error on repeat process message: %v", err)
			continue
		}
		if data.ResultList[0].Ok {
			err = r.repo.Delete(ctx, repeat.Id)
		} else {
			repeat.ApplyNextAttempt(r.defaultStrategy)
			repeat.Error = data.ResultList[0].Message
			err = r.repo.UpdateAttempt(ctx, repeat)
		}
		if err != nil {
			logger.Consumer(logger.App, bag.Consumer, "Failed save to repo after processed: %v", err)
		}
	}
}
