package adminapi

import "context"

type deleteErrorRequest struct {
	Topic string `json:"topic"`
	Group string `json:"group"`
	Error string `json:"error"`
}

func (a *AdminApi) deleteErrorHandler(ctx context.Context, req deleteErrorRequest) (*emptyResponse, error) {
	return &emptyResponse{}, a.service.DeleteFailedByError(ctx, req.Topic, req.Group, req.Error)
}
