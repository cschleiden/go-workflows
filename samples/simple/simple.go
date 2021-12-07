package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cschleiden/go-dt/internal/workflow" // TODO: Remove
	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/memory"
	"github.com/cschleiden/go-dt/pkg/client"
	"github.com/cschleiden/go-dt/pkg/worker"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	mb := memory.NewMemoryBackend()

	// Run worker
	go RunWorker(ctx, mb)

	// Start workflow via client
	c := client.NewTaskHubClient(mb)

	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world")
	if err != nil {
		panic("could not start workflow")
	}

	fmt.Println("Started workflow", wf.GetInstanceID())

	c2 := make(chan os.Signal, 1)
	<-c2
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.NewWorker(mb)

	w.RegisterWorkflow("wf1", Workflow1)

	w.RegisterActivity("a1", Activity1)
	w.RegisterActivity("a2", Activity2)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}

func Workflow1(ctx workflow.Context, msg string) error {
	fmt.Println("Entering Workflow1")
	fmt.Println("\tIsReplaying:", ctx.Replaying())

	// a1, err := workflow.ExecuteActivity(ctx, Activity1)
	// if err != nil {
	// 	panic("error executing activity 1")
	// }

	// r1, err := a1.Get()
	// if err != nil {
	// 	panic("error getting activity 1 result")
	// }
	// fmt.Println("R1 result:", r1)

	// a2, err := workflow.ExecuteActivity(ctx, Activity2)
	// if err != nil {
	// 	panic("error executing activity 1")
	// }

	// r2, err := a2.Get()
	// if err != nil {
	// 	panic("error getting activity 1 result")
	// }
	// fmt.Println("R2 result:", r2)

	// fmt.Println("Leaving Workflow1")

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
