package adminapi

import (
	"context"

	"github.com/prokraft/redbus/internal/app/model"
)

type consumerStatResponse struct {
	List []model.StatConsumer `json:"list"`
}

func (a *AdminApi) consumerStatHandler(ctx context.Context, _ emptyRequest) (*consumerStatResponse, error) {
	list, err := a.service.GetConsumerStats(ctx)
	if err != nil {
		return nil, err
	}
	return &consumerStatResponse{List: list}, nil
}
