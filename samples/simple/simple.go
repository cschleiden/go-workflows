package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/memory"
	"github.com/cschleiden/go-dt/pkg/client"
	"github.com/cschleiden/go-dt/pkg/worker"
	"github.com/cschleiden/go-dt/pkg/workflow"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	mb := memory.NewMemoryBackend()

	// Run worker
	go RunWorker(ctx, mb)

	// Start workflow via client
	c := client.NewTaskHubClient(mb)

	startWorkflow(ctx, c)
	startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
	<-c2
}

func startWorkflow(ctx context.Context, c client.TaskHubClient) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world")
	if err != nil {
		panic("could not start workflow")
	}

	fmt.Println("Started workflow", wf.GetInstanceID())
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

func Workflow1(ctx workflow.Context) error {
	fmt.Println("Entering Workflow1")
	fmt.Println("\tIsReplaying:", ctx.Replaying())

	a1, err := ctx.ExecuteActivity("a1", 35, 12)
	if err != nil {
		panic("error executing activity 1")
	}

	var r1, r2 int
	err = a1.Get(&r1)
	if err != nil {
		panic("error getting activity 1 result")
	}
	fmt.Println("R1 result:", r1)

	a2, err := ctx.ExecuteActivity("a2")
	if err != nil {
		panic("error executing activity 1")
	}

	err = a2.Get(&r2)
	if err != nil {
		panic("error getting activity 1 result")
	}
	fmt.Println("R2 result:", r2)

	fmt.Println("Leaving Workflow1")

	return nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	fmt.Println("Entering Activity1")

	defer func() {
		fmt.Println("Leaving Activity1")
	}()

	return a + b, nil
}

func Activity2(ctx context.Context) (int, error) {
	fmt.Println("Entering Activity2")

	defer func() {
		fmt.Println("Leaving Activity2")
	}()

	return 12, nil
}
