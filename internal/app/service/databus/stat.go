package databus

import (
	"context"

	"github.com/prokraft/redbus/internal/app/model"
)

func (b *DataBus) GetStat(ctx context.Context) (model.Stat, error) {
	repeatCount, err := b.repeater.GetCount(ctx)
	if err != nil {
		return model.Stat{}, err
	}
	return model.Stat{
		ConsumerCount:       b.connStore.GetConsumerCount(),
		ConsumeTopicCount:   b.connStore.GetConsumeTopicCount(),
		RepeatAllCount:      repeatCount.All,
		RepeatFailedCount:   repeatCount.Failed,
		RepeatDeferredCount: repeatCount.Deferred,
	}, nil
}
