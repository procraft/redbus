package repeater

import (
	"context"
	"github.com/sergiusd/redbus/internal/pkg/logger"

	"github.com/sergiusd/redbus/internal/app/model"
)

type Repeater struct {
	repo IRepository
}

type IRepository interface {
	Insert(ctx context.Context, repeat model.Repeat) error
	FindForRepeat(ctx context.Context, topicGroupList model.TopicGroupList) (model.RepeatList, error)
}

func New(repo IRepository) *Repeater {
	return &Repeater{
		repo: repo,
	}
}

func (r *Repeater) Add(ctx context.Context, data model.RepeatData, errorMsg string) error {
	return r.repo.Insert(ctx, model.Repeat{
		Topic:      data.Topic,
		Group:      data.Group,
		ConsumerId: data.ConsumerId,
		MessageId:  data.MessageId,
		Error:      errorMsg,
		Key:        data.Key,
		Data:       data.Message,
		Strategy:   data.Strategy,
	})
}

func (r *Repeater) update(ctx context.Context, repeat *model.Repeat) error {
	return nil
}

func (r *Repeater) Repeat(ctx context.Context) error {
	logger.Info(ctx, "Repeater iteration")
	return nil
}
