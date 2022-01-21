package workflow

import (
	"time"

	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/cschleiden/go-dt/internal/workflow"
	internal "github.com/cschleiden/go-dt/internal/workflow"
)

type SubWorkflowInstanceOptions = internal.SubWorkflowInstanceOptions

func Replaying(ctx Context) bool {
	return internal.Replaying(ctx)
}

func CreateSubWorkflowInstance(ctx Context, options SubWorkflowInstanceOptions, workflow workflow.Workflow, args ...interface{}) Future {
	return internal.CreateSubWorkflowInstance(ctx, options, workflow, args...)
}

func ExecuteActivity(ctx Context, activity workflow.Activity, args ...interface{}) Future {
	return internal.ExecuteActivity(ctx, activity, args...)
}

func ScheduleTimer(ctx Context, delay time.Duration) Future {
	return internal.ScheduleTimer(ctx, delay)
}

func NewSelector() sync.Selector {
	return sync.NewSelector()
}

func NewSignalChannel(ctx Context, name string) Channel {
	return internal.NewSignalChannel(ctx, name)
}
