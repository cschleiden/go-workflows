package main

import (
	"context"
	"errors"
	"time"

	"github.com/cschleiden/go-workflows/activity"
	"github.com/cschleiden/go-workflows/workflow"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Inputs struct {
	Msg   string
	Times int
}

func Workflow1(ctx workflow.Context, msg string, times int, inputs Inputs) (int, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow1", "msg", msg, "times", times, "inputs", inputs)
	defer logger.Debug("Leaving Workflow1")

	tracer := workflow.Tracer(ctx)
	ctx, span := tracer.Start(ctx, "Workflow1 custom span")
	defer span.End()

	_, customSpan := tracer.Start(ctx, "Workflow1 custom inner span", trace.WithAttributes(
		// Add additional
		attribute.String("msg", "hello world"),
	))

	// Do something

	customSpan.End()

	workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)

	workflow.Sleep(ctx, time.Second*1)

	workflow.CreateSubWorkflowInstance[any](ctx, workflow.DefaultSubWorkflowOptions, Subworkflow).Get(ctx)

	workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)

	_, span = workflow.Tracer(ctx).Start(ctx, "Wait-for-signal")
	workflow.NewSignalChannel[string](ctx, "test-signal").Receive(ctx)
	span.End()

	r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, RetriedActivity, 35, 12).Get(ctx)

	return r1, err
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

	ctx, span := otel.Tracer("activity").Start(ctx, "Custom Activity1 span")
	defer span.End()

	_, span = otel.Tracer("activity").Start(ctx, "Another one")
	defer span.End()

	time.Sleep(300 * time.Millisecond)

	return a + b, nil
}

func RetriedActivity(ctx context.Context, a, b int) (int, error) {
	logger := activity.Logger(ctx)

	if activity.Attempt(ctx) < 1 {
		logger.Info("Simulating failure")
		return 0, errors.New("simulated failure")
	}

	return a + b, nil
}
