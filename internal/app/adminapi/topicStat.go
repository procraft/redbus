package adminapi

import (
	"context"

	"github.com/procraft/redbus/internal/app/model"
)

type topicStatResponse struct {
	List []model.StatTopic `json:"list"`
}

func (a *AdminApi) topicStatHandler(ctx context.Context, _ emptyRequest) (*topicStatResponse, error) {
	list, err := a.service.GetTopicStats(ctx)
	if err != nil {
		return nil, err
	}
	return &topicStatResponse{List: list}, nil
}
