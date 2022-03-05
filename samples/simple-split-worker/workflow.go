package simple_split_worker

import (
	"context"
	"log"
	"time"

	"github.com/cschleiden/go-workflows/workflow"
)

func Workflow1(ctx workflow.Context, msg string) error {
	log.Println("Entering Workflow1")
	log.Println("\tWorkflow instance input:", msg)
	log.Println("\tIsReplaying:", workflow.Replaying(ctx))

	defer func() {
		log.Println("Leaving Workflow1")
	}()

	var r1, r2 int
	err := workflow.ExecuteActivity(ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx, &r1)
	if err != nil {
		panic("error getting activity 1 result")
	}
	log.Println("R1 result:", r1)

	log.Println("\tIsReplaying:", workflow.Replaying(ctx))

	err = workflow.ExecuteActivity(ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx, &r2)
	if err != nil {
		panic("error getting activity 1 result")
	}
	log.Println("R2 result:", r2)

	return nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering Activity1")

	time.Sleep(5 * time.Second)

	defer func() {
		log.Println("Leaving Activity1")
	}()

	return a + b, nil
}

func Activity2(ctx context.Context) (int, error) {
	log.Println("Entering Activity2")

	defer func() {
		log.Println("Leaving Activity2")
	}()

	return 12, nil
}
