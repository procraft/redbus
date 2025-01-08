package adminapi

import (
	"context"
	"github.com/prokraft/redbus/internal/app/model"
)

type topicListResponse struct {
	List []model.StatTopic `json:"list"`
}

func (a *AdminApi) topicListHandler(ctx context.Context, _ emptyRequest) (*topicListResponse, error) {
	list, err := a.dataBus.GetTopicList(ctx)
	if err != nil {
		return nil, err
	}
	return &topicListResponse{List: list}, nil
}
