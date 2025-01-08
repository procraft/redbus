package repeater

import (
	"context"
	"sync"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/app/service/connstore"
	"github.com/prokraft/redbus/internal/pkg/logger"
	"github.com/prokraft/redbus/internal/pkg/runtime"
	"github.com/prokraft/redbus/internal/pkg/stream"
)

type Repeater struct {
	defaultStrategy *model.RepeatStrategy
	connStore       IConnStore
	repo            IRepository
	eventSource     IEventSource
}

type IRepository interface {
	Insert(ctx context.Context, repeat model.Repeat) error
	FindForRepeat(ctx context.Context, topicGroupList model.TopicGroupList) (model.RepeatList, error)
	Delete(ctx context.Context, repeatId int64) error
	UpdateAttempt(ctx context.Context, repeat *model.Repeat) error
	GetCount(ctx context.Context) (int, int, error)
	GetStat(ctx context.Context) (model.RepeatStat, error)
	RestartFailed(ctx context.Context, topic, group string) error
}

type IConnStore interface {
	GetConsumerTopicGroupList() model.TopicGroupList
	FindBestConsumerBag(topic model.TopicName, group model.GroupName, id model.ConsumerId) *connstore.ConsumerBag
}

type IEventSource interface {
	Publish(fn func() model.Event)
}

func New(defaultStrategy *model.RepeatStrategy, connStore IConnStore, repo IRepository, eventSource IEventSource) *Repeater {
	return &Repeater{
		defaultStrategy: defaultStrategy,
		connStore:       connStore,
		repo:            repo,
		eventSource:     eventSource,
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
	err := r.repo.Insert(ctx, repeat)
	r.publishEventSource(ctx)
	return err
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
			r.repeatProcessor(ctx, list)
			wg.Done()
		}(consumerRepeatList)
	}
	wg.Wait()
	return nil
}

func (r *Repeater) GetCount(ctx context.Context) (int, int, error) {
	return r.repo.GetCount(ctx)
}

func (r *Repeater) GetStat(ctx context.Context) (model.RepeatStat, error) {
	return r.repo.GetStat(ctx)
}

func (r *Repeater) RestartFailed(ctx context.Context, topic, group string) error {
	return r.repo.RestartFailed(ctx, topic, group)
}

func (r *Repeater) publishEventSource(ctx context.Context) {
	r.eventSource.Publish(func() model.Event {
		allCount, failedCount, _ := r.GetCount(ctx)
		return model.EventRepeater{AllCount: allCount, Failedount: failedCount}
	})
}

func (r *Repeater) repeatProcessor(ctx context.Context, repeatList model.RepeatList) {
	for _, repeat := range repeatList {
		bag := r.connStore.FindBestConsumerBag(repeat.Topic, repeat.Group, repeat.ConsumerId)
		if bag == nil || bag.Consumer.GetState() != model.ConsumerStateConnected {
			continue
		}
		data, err := stream.New(bag.Server).ProcessMessageList(logger.App, bag.Consumer, model.MessageList{{Id: repeat.MessageId, Value: repeat.Data}})
		if err != nil {
			logger.Error(ctx, "Error on repeat process message: %v", err)
			continue
		}
		if data.ResultList[0].Ok {
			err = r.repo.Delete(ctx, repeat.Id)
			r.publishEventSource(ctx)
		} else {
			repeat.ApplyNextAttempt(r.defaultStrategy)
			repeat.Error = data.ResultList[0].Message
			err = r.repo.UpdateAttempt(ctx, repeat)
			if repeat.FinishedAt != nil {
				r.publishEventSource(ctx)
			}
		}
		if err != nil {
			logger.Consumer(logger.App, bag.Consumer, "Failed save to repo after processed: %v", err)
		}
	}
}
