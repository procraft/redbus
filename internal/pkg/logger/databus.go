package logger

import (
	"context"
	"strings"

	"github.com/procraft/redbus/internal/app/model"
)

func Consumer(ctx context.Context, c model.IConsumer, message string, args ...any) {
	consumerLog(ctx, LevelInfo, c, message, args...)
}

// ConsumerWarning — то же, что Consumer, уровнем warning: ошибки чтения kafka должны быть видны
// фильтром по уровню (метка l=WARN в Loki).
func ConsumerWarning(ctx context.Context, c model.IConsumer, message string, args ...any) {
	consumerLog(ctx, LevelWarning, c, message, args...)
}

func consumerLog(ctx context.Context, level Level, c model.IConsumer, message string, args ...any) {
	if c == nil {
		Log(ctx, level, "[%v] "+message+"\n", args...)
		return
	}
	args = append([]any{strings.Join(c.GetHosts(), ","), c.GetTopic(), c.GetGroup(), c.GetID()}, args...)
	Log(ctx, level, "[%v/%v/%v/%v] "+message+"\n", args...)
}

func Produce(ctx context.Context, topic model.TopicName, message string, args ...any) {
	args = append([]any{ /*strings.Join(c.GetHosts(), ","), */ topic}, args...)
	Info(ctx, "[%v] "+message+"\n", args...)
}
