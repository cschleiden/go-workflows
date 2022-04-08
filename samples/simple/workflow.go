package main

import (
	"context"

	"github.com/cschleiden/go-workflows/activity"
	"github.com/cschleiden/go-workflows/workflow"
)

type Inputs struct {
	Msg   string
	Times int
}

func Workflow1(ctx workflow.Context, msg string, times int, inputs Inputs) (int, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow1", "msg", msg, "times", times, "inputs", inputs)
	defer logger.Debug("Leaving Workflow1")

	r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)
	if err != nil {
		panic("error getting activity 1 result")
	}
	logger.Debug("R1 result", "r1", r1)

	r2, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx)
	if err != nil {
		panic("error getting activity 1 result")
	}
	logger.Debug("R2 result", "r2", r2)

	return r1 + r2, nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	logger := activity.Logger(ctx)
	logger.Debug("Entering Activity1")
	defer logger.Debug("Leaving Activity1")

	// time.Sleep(5 * time.Second)

	return a + b, nil
}

func Activity2(ctx context.Context) (int, error) {
	logger := activity.Logger(ctx)
	logger.Debug("Entering Activity2")
	defer logger.Debug("Leaving Activity2")

	// time.Sleep(1 * time.Second)

	return 12, nil
}
