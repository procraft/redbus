package adminapi

import "context"

type repeatErrorRequest struct {
	Topic           string `json:"topic"`
	Group           string `json:"group"`
	Error           string `json:"error"`
	LookbackSeconds int64  `json:"lookbackSeconds"`
}

func (a *AdminApi) repeatErrorHandler(ctx context.Context, req repeatErrorRequest) (*emptyResponse, error) {
	return &emptyResponse{}, a.service.RestartFailedByError(
		ctx,
		req.Topic,
		req.Group,
		req.Error,
		req.LookbackSeconds,
	)
}
