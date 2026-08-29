package adminapi

import (
	"context"
)

type healthResponse struct {
	Success bool `json:"success"`
}

func (a *AdminApi) healthHandler(ctx context.Context, _ emptyRequest) (*healthResponse, error) {
	if err := a.service.Health(ctx); err != nil {
		return nil, err
	}
	return &healthResponse{Success: true}, nil
}

func (a *AdminApi) liveHandler(_ context.Context, _ emptyRequest) (*healthResponse, error) {
	return &healthResponse{Success: true}, nil
}
