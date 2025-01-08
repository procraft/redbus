package logger

import (
	"context"
	"strings"

	"github.com/prokraft/redbus/internal/app/model"
)

func Consumer(ctx context.Context, c model.IConsumer, message string, args ...any) {
	if c == nil {
		Info(ctx, "[%v] "+message+"\n", args...)
		return
	}
	args = append([]any{strings.Join(c.GetHosts(), ","), c.GetTopic(), c.GetGroup(), c.GetID()}, args...)
	Info(ctx, "[%v/%v/%v/%v] "+message+"\n", args...)
}

func Produce(ctx context.Context, topic model.TopicName, message string, args ...any) {
	args = append([]any{ /*strings.Join(c.GetHosts(), ","), */ topic}, args...)
	Info(ctx, "[%v] "+message+"\n", args...)
}
