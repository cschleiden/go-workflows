package main

import (
	"context"
	"time"

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

	wg := workflow.NewWaitGroup()

	for i := 0; i < times; i++ {
		wg.Add(1)
		i := i
		workflow.Go(ctx, func(ctx workflow.Context) {
			_, err := workflow.CreateSubWorkflowInstance[int](ctx, workflow.DefaultSubWorkflowOptions, RunJob, i+1).Get(ctx)
			if err != nil {
				logger.Error("error getting subworkflow result", "err", err)
				panic("error getting subworkflow result")
			}

			wg.Done()
		})
	}

	wg.Wait(ctx)

	return 42, nil
}

func RunJob(ctx workflow.Context, count int) (int, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering RunJob")
	defer logger.Debug("Leaving RunJob")

	wg := workflow.NewWaitGroup()

	for i := 0; i < count; i++ {
		wg.Add(1)
		workflow.Go(ctx, func(ctx workflow.Context) {
			_, err := workflow.CreateSubWorkflowInstance[int](ctx, workflow.DefaultSubWorkflowOptions, RunStep).Get(ctx)
			if err != nil {
				logger.Error("error getting subworkflow result", "err", err)
				panic("error getting subworkflow result")
			}

			wg.Done()
		})
	}

	wg.Wait(ctx)

	return 42, nil
}

func RunStep(ctx workflow.Context) (int, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering RunStep")
	defer logger.Debug("Leaving RunStep")

	r, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)
	if err != nil {
		panic("error getting activity 1 result")
	}
	logger.Debug("R1 result", "r1", r)

	return r, nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	logger := activity.Logger(ctx)
	logger.Debug("Entering Activity1")
	defer logger.Debug("Leaving Activity1")

	time.Sleep(50 * time.Second)

	return a + b, nil
}
