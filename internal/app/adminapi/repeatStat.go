package adminapi

import (
	"context"
	"time"
)

type repeatStatResponse struct {
	List []repeatStatItemResponse `json:"list"`
}

type repeatStatItemResponse struct {
	Topic       string                    `json:"topic"`
	Group       string                    `json:"group"`
	AllCount    int                       `json:"allCount"`
	FailedCount int                       `json:"failedCount"`
	LastError   string                    `json:"lastError"`
	Errors      []repeatErrorStatResponse `json:"errors"`
}

type repeatErrorStatResponse struct {
	Error         string    `json:"error"`
	FailedCount   int       `json:"failedCount"`
	FirstFailedAt time.Time `json:"firstFailedAt"`
	LastFailedAt  time.Time `json:"lastFailedAt"`
}

func (a *AdminApi) repeatStatHandler(ctx context.Context, _ emptyRequest) (*repeatStatResponse, error) {
	stat, err := a.service.GetRetryStats(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]repeatStatItemResponse, 0, len(stat))
	for _, item := range stat {
		errors := make([]repeatErrorStatResponse, 0, len(item.Errors))
		for _, errorStat := range item.Errors {
			errors = append(errors, repeatErrorStatResponse{
				Error:         errorStat.Error,
				FailedCount:   errorStat.FailedCount,
				FirstFailedAt: errorStat.FirstFailedAt,
				LastFailedAt:  errorStat.LastFailedAt,
			})
		}
		list = append(list, repeatStatItemResponse{
			Topic:       item.Topic,
			Group:       item.Group,
			AllCount:    item.AllCount,
			FailedCount: item.FailedCount,
			LastError:   item.LastError,
			Errors:      errors,
		})
	}
	return &repeatStatResponse{List: list}, nil
}
