package repeater

import (
	"context"

	"github.com/sergiusd/redbus/internal/app/model"
)

type Repeater struct {
	repo IRepository
}

type IRepository interface {
	Insert(ctx context.Context, repeat model.Repeat) error
}

func New(repo IRepository) *Repeater {
	return &Repeater{
		repo: repo,
	}
}

func (r *Repeater) Add(ctx context.Context, topic, group, consumerId string, messageKey, message []byte, messageId, errorMessage string) error {
	return r.repo.Insert(ctx, model.Repeat{
		Topic:      topic,
		Group:      group,
		ConsumerId: consumerId,
		MessageId:  messageId,
		Error:      errorMessage,
		Key:        messageKey,
		Data:       message,
	})
}

func (r *Repeater) Update(ctx context.Context, repeat *model.Repeat) error {
	return nil
}

func (r *Repeater) FindForRepeat(ctx context.Context) (model.RepeatList, error) {
	return nil, nil
}
