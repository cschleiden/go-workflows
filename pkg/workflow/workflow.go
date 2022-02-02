package workflow

import (
	"time"

	"github.com/cschleiden/go-dt/internal/sync"
	internal "github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/core"
)

type SubWorkflowInstanceOptions = internal.SubWorkflowInstanceOptions

func Replaying(ctx Context) bool {
	return internal.Replaying(ctx)
}

type Workflow = internal.Workflow

func CreateSubWorkflowInstance(ctx Context, options SubWorkflowInstanceOptions, workflow Workflow, args ...interface{}) Future {
	return internal.CreateSubWorkflowInstance(ctx, options, workflow, args...)
}

type Activity = internal.Activity

func ExecuteActivity(ctx Context, activity Activity, args ...interface{}) Future {
	return internal.ExecuteActivity(ctx, activity, args...)
}

func ScheduleTimer(ctx Context, delay time.Duration) Future {
	return internal.ScheduleTimer(ctx, delay)
}

func NewSignalChannel(ctx Context, name string) Channel {
	return internal.NewSignalChannel(ctx, name)
}

func WorkflowInstance(ctx Context) core.WorkflowInstance {
	return internal.WorkflowInstance(ctx)
}

func Now(ctx Context) time.Time {
	return internal.Now(ctx)
}

func Sleep(ctx Context, d time.Duration) error {
	return internal.Sleep(ctx, d)
}

func Go(ctx Context, f func(ctx Context)) {
	sync.Go(ctx, f)
}

func NewSelector() sync.Selector {
	return sync.NewSelector()
}

func NewChannel() Channel {
	return sync.NewChannel()
}

func NewBufferedChannel(size int) Channel {
	return sync.NewBufferedChannel(size)
}
