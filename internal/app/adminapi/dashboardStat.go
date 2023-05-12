package adminapi

import (
	"context"
)

type dashboardStatResponse struct {
	ConsumeTopicCount int `json:"consumeTopicCount"`
	ConsumerCount     int `json:"consumerCount"`
	RepeatAllCount    int `json:"repeatAllCount"`
	RepeatFailedCount int `json:"repeatFailedCount"`
}

func (a *AdminApi) dashboardStatHandler(ctx context.Context, _ emptyRequest) (*dashboardStatResponse, error) {
	stat, err := a.dataBus.GetStat(ctx)
	if err != nil {
		return nil, err
	}
	return &dashboardStatResponse{
		ConsumeTopicCount: stat.ConsumeTopicCount,
		ConsumerCount:     stat.ConsumerCount,
		RepeatAllCount:    stat.RepeatAllCount,
		RepeatFailedCount: stat.RepeatFailedCount,
	}, nil
}
