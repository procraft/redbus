package adminapi

import "context"

type repeatTopicGroupSinceRequest struct {
	Topic           string `json:"topic"`
	Group           string `json:"group"`
	LookbackSeconds int64  `json:"lookbackSeconds"`
}

func (a *AdminApi) repeatTopicGroupSinceHandler(ctx context.Context, req repeatTopicGroupSinceRequest) (*emptyResponse, error) {
	return &emptyResponse{}, a.service.RestartFailedSince(
		ctx,
		req.Topic,
		req.Group,
		req.LookbackSeconds,
	)
}
