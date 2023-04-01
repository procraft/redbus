package logger

type Logger interface {
	Debug(msg string, args ...any) error
	Info(msg string, args ...any) error
	Warn(msg string, args ...any) error
	Error(msg string, args ...any) error

	WithKV(kv ...any) Logger
}
