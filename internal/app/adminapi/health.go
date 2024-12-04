package adminapi

import (
	"context"
	"github.com/prokraft/redbus/internal/pkg/logger"
)

type healthResponse struct {
	Success bool `json:"success"`
}

func (a *AdminApi) healthHandler(ctx context.Context, _ emptyRequest) (*healthResponse, error) {
	logger.Info(ctx, "Health called")
	return &healthResponse{Success: true}, nil
}
