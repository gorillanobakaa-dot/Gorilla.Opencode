package logging

import (
	"context"
	"io"
	"log/slog"
	"slices"

	"github.com/kujtimiihoxha/termai/internal/pubsub"
	"golang.org/x/exp/maps"
)

const DefaultLevel = "info"

const (
	persistKeyArg  = "$persist"
	PersistTimeArg = "$persist_time"
)

var levels = map[string]slog.Level{
	"debug":      slog.LevelDebug,
	DefaultLevel: slog.LevelInfo,
	"warn":       slog.LevelWarn,
	"error":      slog.LevelError,
}

func ValidLevels() []string {
	keys := maps.Keys(levels)
	slices.SortFunc(keys, func(a, b string) int {
		if a == DefaultLevel {
			return -1
		}
		if b == DefaultLevel {
			return 1
		}
		if a < b {
			return -1
		}
		return 1
	})
	return keys
}

func NewLogger(opts Options) Interface {
	logger := &Logger{}
	broker := pubsub.NewBroker[LogMessage]()
	writer := &writer{
		messages: []LogMessage{},
		Broker:   broker,
	}

	handler := slog.NewTextHandler(
		io.MultiWriter(writer),
		&slog.HandlerOptions{
			Level: slog.Level(levels[opts.Level]),
		},
	)
	logger.logger = slog.New(handler)
	logger.writer = writer

	return logger
}

type Options struct {
	Level string
}

type Logger struct {
	logger *slog.Logger
	writer *writer
}

func (l *Logger) SetLevel(level string) {
	if _, ok := levels[level]; !ok {
		level = DefaultLevel
	}
	handler := slog.NewTextHandler(
		io.MultiWriter(l.writer),
		&slog.HandlerOptions{
			Level: levels[level],
		},
	)
	l.logger = slog.New(handler)
}

// PersistDebug implements Interface.
func (l *Logger) PersistDebug(msg string, args ...any) {
	args = append(args, persistKeyArg, true)
	l.Debug(msg, args...)
}

// PersistError implements Interface.
func (l *Logger) PersistError(msg string, args ...any) {
	args = append(args, persistKeyArg, true)
	l.Error(msg, args...)
}

// PersistInfo implements Interface.
func (l *Logger) PersistInfo(msg string, args ...any) {
	args = append(args, persistKeyArg, true)
	l.Info(msg, args...)
}

// PersistWarn implements Interface.
func (l *Logger) PersistWarn(msg string, args ...any) {
	args = append(args, persistKeyArg, true)
	l.Warn(msg, args...)
}

func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

func (l *Logger) List() []LogMessage {
	return l.writer.messages
}

func (l *Logger) Get(id string) (LogMessage, error) {
	for _, msg := range l.writer.messages {
		if msg.ID == id {
			return msg, nil
		}
	}
	return LogMessage{}, io.EOF
}

func (l *Logger) Subscribe(ctx context.Context) <-chan pubsub.Event[LogMessage] {
	return l.writer.Subscribe(ctx)
}
