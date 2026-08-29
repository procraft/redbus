package adminapp

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/config"

	"github.com/stretchr/testify/require"
)

func TestStatPollerSkipsSnapshotWithoutEventConsumers(t *testing.T) {
	conf := &config.Config{}
	conf.Admin.PollInterval = model.NewDuration(time.Millisecond)

	var calls atomic.Int32
	app := &App{
		conf:                conf,
		eventConsumersCount: func() int { return 0 },
		getStateSnapshot: func(context.Context) (model.Stat, error) {
			calls.Add(1)
			return model.Stat{}, nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := app.getStatPoller(ctx)()
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Zero(t, calls.Load())
}
