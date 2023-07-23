package adminapi

import "context"

type repeatStatResponse struct {
	List []repeatStatItemResponse `json:"list"`
}

type repeatStatItemResponse struct {
	Topic       string `json:"topic"`
	Group       string `json:"group"`
	AllCount    int    `json:"allCount"`
	FailedCount int    `json:"failedCount"`
	LastError   string `json:"lastError"`
}

func (a *AdminApi) repeatStatHandler(ctx context.Context, _ emptyRequest) (*repeatStatResponse, error) {
	stat, err := a.repeater.GetStat(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]repeatStatItemResponse, 0, len(stat))
	for _, item := range stat {
		list = append(list, repeatStatItemResponse{
			Topic:       item.Topic,
			Group:       item.Group,
			AllCount:    item.AllCount,
			FailedCount: item.FailedCount,
			LastError:   item.LastError,
		})
	}
	return &repeatStatResponse{List: list}, nil
}
