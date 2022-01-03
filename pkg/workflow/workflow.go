package workflow

import (
	"context"
	"time"

	"github.com/cschleiden/go-dt/internal/sync"
	internal "github.com/cschleiden/go-dt/internal/workflow"
)

func Replaying(ctx context.Context) bool {
	return internal.Replaying(ctx)
}

func ExecuteActivity(ctx context.Context, name string, args ...interface{}) (sync.Future, error) {
	return internal.ExecuteActivity(ctx, name, args...)
}

func ScheduleTimer(ctx context.Context, delay time.Duration) (sync.Future, error) {
	return internal.ScheduleTimer(ctx, delay)
}

func NewSelector() sync.Selector {
	return sync.NewSelector()
}

func NewSignalChannel(ctx context.Context, name string) sync.Channel {
	return internal.NewSignalChannel(ctx, name)
}
