package adminapi

import (
	"context"
)

type healthResponse struct {
	Success bool `json:"success"`
}

func (a *AdminApi) healthHandler(ctx context.Context, _ emptyRequest) (*healthResponse, error) {
	return &healthResponse{Success: true}, nil
}
