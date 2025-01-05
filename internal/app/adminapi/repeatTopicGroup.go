package adminapi

import (
	"context"
)

type repeatTopicGroupRequest struct {
	Topic string `json:"topic"`
	Group string `json:"group"`
}

func (a *AdminApi) repeatTopicGroupHandler(ctx context.Context, req repeatTopicGroupRequest) (*emptyResponse, error) {
	return &emptyResponse{}, a.repeater.RestartFailed(ctx, req.Topic, req.Group)
}
