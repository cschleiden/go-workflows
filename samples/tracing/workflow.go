package main

import (
	"context"
	"time"

	"github.com/cschleiden/go-workflows/activity"
	"github.com/cschleiden/go-workflows/workflow"
	"go.opentelemetry.io/otel"
)

type Inputs struct {
	Msg   string
	Times int
}

func Workflow1(ctx workflow.Context, msg string, times int, inputs Inputs) (int, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow1", "msg", msg, "times", times, "inputs", inputs)
	defer logger.Debug("Leaving Workflow1")

	workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)

	workflow.Sleep(ctx, time.Second*1)

	workflow.CreateSubWorkflowInstance[any](ctx, workflow.DefaultSubWorkflowOptions, Subworkflow).Get(ctx)

	workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)

	r1, _ := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)

	return r1, nil
}

func Subworkflow(ctx workflow.Context) error {
	workflow.Sleep(ctx, time.Millisecond*500)

	workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)

	return nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	logger := activity.Logger(ctx)
	logger.Debug("Entering Activity1")
	defer logger.Debug("Leaving Activity1")

	_, span := otel.Tracer("activity").Start(ctx, "Activity1")
	defer span.End()

	time.Sleep(300 * time.Millisecond)

	return a + b, nil
}
