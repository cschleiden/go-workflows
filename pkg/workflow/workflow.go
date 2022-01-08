package workflow

import (
	"time"

	"github.com/cschleiden/go-dt/internal/sync"
	internal "github.com/cschleiden/go-dt/internal/workflow"
)

func Replaying(ctx Context) bool {
	return internal.Replaying(ctx)
}

func CreateSubWorkflowInstance(ctx Context, name string, args ...interface{}) (Future, error) {
	return internal.CreateSubWorkflowInstance(ctx, name, args...)
}

func ExecuteActivity(ctx Context, name string, args ...interface{}) (Future, error) {
	return internal.ExecuteActivity(ctx, name, args...)
}

func ScheduleTimer(ctx Context, delay time.Duration) (Future, error) {
	return internal.ScheduleTimer(ctx, delay)
}

func NewSelector() sync.Selector {
	return sync.NewSelector()
}

func NewSignalChannel(ctx Context, name string) Channel {
	return internal.NewSignalChannel(ctx, name)
}
