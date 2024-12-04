package databus

import (
	"context"
	"github.com/prokraft/redbus/internal/app/model"
)

func (b *DataBus) GetStat(ctx context.Context) (model.DataBusStat, error) {
	repeatAllCount, repeatFailedCount, err := b.repeater.GetCount(ctx)
	if err != nil {
		return model.DataBusStat{}, err
	}
	return model.DataBusStat{
		ConsumerCount:     b.connStore.GetConsumerCount(),
		ConsumeTopicCount: b.connStore.GetConsumeTopicCount(),
		RepeatAllCount:    repeatAllCount,
		RepeatFailedCount: repeatFailedCount,
	}, nil
}
