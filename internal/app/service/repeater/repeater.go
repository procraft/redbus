package repeater

import (
	"context"
	"sync"
	"time"

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
	metrics         IMetrics
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

type IMetrics interface {
	ObserveRetryEnqueued(topic, group, result string)
	ObserveRetryAttempt(topic, group, outcome string)
	ObserveRetrySkipped(topic, group, reason string)
	ObserveRepeaterRun(result string, messages int, duration time.Duration)
}

func New(defaultStrategy *model.RepeatStrategy, connStore IConnStore, repo IRepository, metrics IMetrics) *Repeater {
	return &Repeater{
		defaultStrategy: defaultStrategy,
		connStore:       connStore,
		repo:            repo,
		metrics:         metrics,
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
		Headers:    data.Headers,
		Strategy:   data.Strategy,
		CreatedAt:  runtime.Now(),
	}
	repeat.SetZeroAttempt(r.defaultStrategy)
	err := r.repo.Insert(ctx, repeat)
	result := "success"
	if err != nil {
		result = "error"
	}
	r.metrics.ObserveRetryEnqueued(string(data.Topic), string(data.Group), result)
	return err
}

func (r *Repeater) Repeat(ctx context.Context) error {
	startedAt := time.Now()
	result := "success"
	messageCount := 0
	defer func() {
		r.metrics.ObserveRepeaterRun(result, messageCount, time.Since(startedAt))
	}()
	topicGroupList := r.connStore.GetConsumerTopicGroupList()
	repeatList, err := r.repo.FindForRepeat(ctx, topicGroupList)
	if err != nil {
		result = "error"
		return err
	}
	messageCount = len(repeatList)
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

func (r *Repeater) repeatProcessor(ctx context.Context, repeatList model.RepeatList) {
	for _, repeat := range repeatList {
		bag := r.connStore.FindBestConsumerBag(repeat.Topic, repeat.Group, repeat.ConsumerId)
		if bag == nil {
			r.metrics.ObserveRetrySkipped(string(repeat.Topic), string(repeat.Group), "no_consumer")
			continue
		}
		if bag.Consumer.GetState() != model.ConsumerStateConnected {
			r.metrics.ObserveRetrySkipped(string(repeat.Topic), string(repeat.Group), "consumer_not_connected")
			continue
		}
		data, err := stream.New(bag.Server).ProcessMessageList(
			logger.App,
			bag.Consumer,
			model.MessageList{{Id: repeat.MessageId, Value: repeat.Data, Headers: repeat.Headers}},
		)
		if err != nil {
			r.metrics.ObserveRetryAttempt(string(repeat.Topic), string(repeat.Group), "stream_error")
			logger.Error(ctx, "Error on repeat process message: %v", err)
			continue
		}
		if len(data.ResultList) == 0 {
			logger.Error(ctx, "Error on repeat process message: empty result list")
			repeat.ApplyNextAttempt(r.defaultStrategy)
			repeat.Error = "empty result list"
			err = r.repo.UpdateAttempt(ctx, repeat)
			if err != nil {
				r.metrics.ObserveRetryAttempt(string(repeat.Topic), string(repeat.Group), "persistence_error")
				logger.Consumer(logger.App, bag.Consumer, "Failed save to repo after processed: %v", err)
			} else {
				r.metrics.ObserveRetryAttempt(string(repeat.Topic), string(repeat.Group), repeatOutcome(repeat))
			}
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
			r.metrics.ObserveRetryAttempt(string(repeat.Topic), string(repeat.Group), "persistence_error")
			logger.Consumer(logger.App, bag.Consumer, "Failed save to repo after processed: %v", err)
		} else if data.ResultList[0].Ok {
			r.metrics.ObserveRetryAttempt(string(repeat.Topic), string(repeat.Group), "success")
		} else {
			r.metrics.ObserveRetryAttempt(string(repeat.Topic), string(repeat.Group), repeatOutcome(repeat))
		}
	}
}

func repeatOutcome(repeat *model.Repeat) string {
	if repeat.FinishedAt != nil {
		return "exhausted"
	}
	return "failed"
}
