package main

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/activity"
	"github.com/cschleiden/go-workflows/workflow"
)

func Workflow1(ctx workflow.Context) (string, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow1")
	defer logger.Debug("Leaving Workflow1")

	data := myValuesWf(ctx)

	r1, err := workflow.ExecuteActivity[string](ctx, workflow.DefaultActivityOptions, Activity1).Get(ctx)
	if err != nil {
		panic("error getting activity 1 result")
	}
	logger.Debug("R1 result", "r1", r1)

	ctx = withMyValuesWf(ctx, &myData{
		Name:  "world",
		Count: data.Count + 1,
	})

	r2, err := workflow.ExecuteActivity[string](ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx)
	if err != nil {
		panic("error getting activity 2 result")
	}
	logger.Debug("R2 result", "r2", r2)

	return fmt.Sprintf("%s-%s-%s", data.Name, r1, r2), nil
}

func Activity1(ctx context.Context) (string, error) {
	logger := activity.Logger(ctx)
	logger.Debug("Entering Activity1")
	defer logger.Debug("Leaving Activity1")

	d := myValues(ctx)
	return fmt.Sprintf("%s%d", d.Name, d.Count), nil
}

func Activity2(ctx context.Context) (string, error) {
	logger := activity.Logger(ctx)
	logger.Debug("Entering Activity2")
	defer logger.Debug("Leaving Activity2")

	d := myValues(ctx)
	return fmt.Sprintf("%s%d", d.Name, d.Count), nil
}
