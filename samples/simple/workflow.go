package main

import (
	"context"
	"log"
	"time"

	"github.com/cschleiden/go-workflows/samples"
	"github.com/cschleiden/go-workflows/workflow"
)

type Inputs struct {
	Msg   string
	Times int
}

func Workflow1(ctx workflow.Context, msg string, times int, inputs Inputs) (int, error) {
	samples.Trace(ctx, "Entering Workflow1", msg, times, inputs)

	defer samples.Trace(ctx, "Leaving Workflow1")

	r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)
	if err != nil {
		panic("error getting activity 1 result")
	}
	samples.Trace(ctx, "R1 result:", r1)

	r2, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx)
	if err != nil {
		panic("error getting activity 1 result")
	}
	samples.Trace(ctx, "R2 result:", r2)

	return r1 + r2, nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering Activity1")
	defer log.Println("Leaving Activity1")

	time.Sleep(5 * time.Second)

	return a + b, nil
}

func Activity2(ctx context.Context) (int, error) {
	log.Println("Entering Activity2")
	defer log.Println("Leaving Activity2")

	time.Sleep(1 * time.Second)

	return 12, nil
}
