package adminapi

import (
	"context"
)

type dashboardStatResponse struct {
	ConsumeTopicCount int `json:"consumeTopicCount"`
	ConsumerCount     int `json:"consumerCount"`
	RepeatAllCount    int `json:"repeatAllCount"`
	RepeatFailedCount int `json:"repeatFailedCount"`
	// RepeatDeferredCount is part of RepeatAllCount and never part of RepeatFailedCount.
	RepeatDeferredCount int `json:"repeatDeferredCount"`
}

func (a *AdminApi) dashboardStatHandler(ctx context.Context, _ emptyRequest) (*dashboardStatResponse, error) {
	stat, err := a.service.GetStateSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return &dashboardStatResponse{
		ConsumeTopicCount:   stat.ConsumeTopicCount,
		ConsumerCount:       stat.ConsumerCount,
		RepeatAllCount:      stat.RepeatAllCount,
		RepeatFailedCount:   stat.RepeatFailedCount,
		RepeatDeferredCount: stat.RepeatDeferredCount,
	}, nil
}
