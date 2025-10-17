package main

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/activity"
	"github.com/cschleiden/go-workflows/workflow"
)

type Inputs struct {
	Msg    string
	Result int
}

func Workflow1(ctx workflow.Context, start, times int, inputs Inputs) (int, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow1", "start", start, "times", times, "inputs", inputs)
	defer logger.Debug("Leaving Workflow1")

	for i := start; i < times; i++ {
		r, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 7).Get(ctx)
		if err != nil {
			return 0, fmt.Errorf("error getting activity 1 result: %w", err)
		}
		logger.Debug("Activity result", "r", r)

		inputs.Result = inputs.Result + r

		_, err = workflow.CreateSubWorkflowInstance[int](ctx, workflow.DefaultSubWorkflowOptions, SubWorkflow, 1, 5).Get(ctx)
		if err != nil {
			return 0, fmt.Errorf("error getting sub workflow result: %w", err)
		}

		// Check history length and continue as new if it's getting large
		// This is useful to avoid hitting history size limits
		info := workflow.GetWorkflowInstanceInfo(ctx)
		logger.Debug("Workflow instance info", "historyLength", info.HistoryLength)

		if i < 3 {
			return r, workflow.ContinueAsNew(ctx, i+1, times, inputs)
		}
	}

	return inputs.Result, nil
}

func SubWorkflow(ctx workflow.Context, start, times int) (int, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering SubWorkflow", "start", start, "times", times)
	defer logger.Debug("Leaving SubWorkflow")

	var i int
	for i = start; i < times; i++ {
		logger.Debug("Iteration", "i", i)

		if i < 3 {
			return 0, workflow.ContinueAsNew(ctx, i+1, times)
		}
	}

	return i, nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	logger := activity.Logger(ctx)
	logger.Debug("Entering Activity1")
	defer logger.Debug("Leaving Activity1")

	return a + b, nil
}
