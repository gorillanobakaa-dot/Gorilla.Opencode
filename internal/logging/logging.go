package logging

import (
	"context"

	"github.com/kujtimiihoxha/termai/internal/pubsub"
)

type Interface interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Subscribe(ctx context.Context) <-chan pubsub.Event[LogMessage]

	PersistDebug(msg string, args ...any)
	PersistInfo(msg string, args ...any)
	PersistWarn(msg string, args ...any)
	PersistError(msg string, args ...any)
	List() []LogMessage

	SetLevel(level string)
}
