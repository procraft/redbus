package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConsumeLimitsBatchTimeoutScalesWithBatch(t *testing.T) {
	limits := ConsumeLimits{PerMessage: 60 * time.Second, Slack: 30 * time.Second, Max: time.Hour}

	require.Equal(t, 90*time.Second, limits.BatchTimeout(1))
	require.Equal(t, 90*time.Second, limits.BatchTimeout(0), "пустой батч считается как один")
	require.Equal(t, 330*time.Second, limits.BatchTimeout(5))
}

func TestConsumeLimitsBatchTimeoutIsCapped(t *testing.T) {
	limits := ConsumeLimits{PerMessage: time.Minute, Slack: time.Minute, Max: 5 * time.Minute}

	require.Equal(t, 5*time.Minute, limits.BatchTimeout(100))
}

func TestConsumeLimitsBatchTimeoutDisabledWithoutPerMessage(t *testing.T) {
	limits := ConsumeLimits{Slack: time.Minute, Max: time.Hour}

	require.Zero(t, limits.BatchTimeout(3), "без бюджета на сообщение ожидание не ограничивается")
}

func TestConsumeLimitsWithPerMessageKeepsServerValueWhenClientSilent(t *testing.T) {
	limits := ConsumeLimits{PerMessage: 60 * time.Second, Slack: time.Second, Max: time.Hour}

	require.Equal(t, 60*time.Second, limits.WithPerMessage(0).PerMessage)
	require.Equal(t, 60*time.Second, limits.WithPerMessage(-time.Second).PerMessage)
	require.Equal(t, 5*time.Minute, limits.WithPerMessage(5*time.Minute).PerMessage)
}
