package main

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-dt/internal/client"
	"github.com/cschleiden/go-dt/internal/worker"
	workflow "github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/memory"
)

func main() {
	mb := memory.NewMemoryBackend()

	// Run worker
	go RunWorker(mb)

	// Start workflow via client
	client := client.NewTaskHubClient(mb)
	if err := client.StartWorkflow(context.Background(), Workflow1); err != nil {
		panic("could not start workflow")
	}

	// TODO: Wait until workflow is done or program is canceled
}

func RunWorker(mb backend.Backend) {
	w := worker.NewWorker(mb)

	w.RegisterWorkflow("wf1", Workflow1)

	w.RegisterActivity("a1", Activity1)
	w.RegisterActivity("a2", Activity2)

	if err := w.Start(); err != nil {
		panic("could not start worker")
	}
}

func Workflow1(ctx workflow.Context, t interface{}) error {
	fmt.Println("Entering Workflow1")
	fmt.Println("\tIsReplaying:", ctx.IsReplaying())

	a1, err := workflow.ExecuteActivity(ctx, Activity1)
	if err != nil {
		panic("error executing activity 1")
	}

	r1, err := a1.Get()
	if err != nil {
		panic("error getting activity 1 result")
	}
	fmt.Println("R1 result:", r1)

	a2, err := workflow.ExecuteActivity(ctx, Activity2)
	if err != nil {
		panic("error executing activity 1")
	}

	r2, err := a2.Get()
	if err != nil {
		panic("error getting activity 1 result")
	}
	fmt.Println("R2 result:", r2)

	fmt.Println("Leaving Workflow1")

	return nil
}

func Activity1(ctx workflow.Context) (int, error) {
	fmt.Println("Entering Activity1")

	return 42, nil
}

func Activity2(ctx workflow.Context) (string, error) {
	fmt.Println("Entering Activity2")

	return "hello", nil
}
