package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/memory"
	"github.com/cschleiden/go-dt/pkg/client"
	"github.com/cschleiden/go-dt/pkg/worker"
	"github.com/cschleiden/go-dt/pkg/workflow"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	mb := memory.NewMemoryBackend()

	// Run worker
	go RunWorker(ctx, mb)

	// Start workflow via client
	c := client.New(mb)

	startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
	<-c2

	cancel()
}

func startWorkflow(ctx context.Context, c client.Client) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world")
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.GetInstanceID())

	time.Sleep(2 * time.Second)

	err = c.CancelWorkflowInstance(ctx, wf)
	if err != nil {
		panic("could not cancel workflow")
	}

	log.Println("Cancelled workflow", wf.GetInstanceID())
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb)

	w.RegisterWorkflow(Workflow1)
	w.RegisterActivity(ActivityCancel)
	w.RegisterActivity(ActivitySkip)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}

func Workflow1(ctx workflow.Context, msg string) (string, error) {
	log.Println("Entering Workflow1")
	log.Println("\tWorkflow instance input:", msg)
	log.Println("\tIsReplaying:", workflow.Replaying(ctx))

	defer func() {
		log.Println("Leaving Workflow1")
	}()

	var r1 int
	if err := workflow.ExecuteActivity(ctx, ActivityCancel, 1, 2).Get(ctx, &r1); err != nil {
		log.Println("error getting activity 1 result")
	}

	var r2 int
	if err := workflow.ExecuteActivity(ctx, ActivitySkip, 1, 2).Get(ctx, &r2); err != nil {
		log.Println("error getting activity 2 result")
	}

	log.Println("Workflow finished")
	return "result", nil
}

func ActivityCancel(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering ActivityCancel")
	defer log.Println("Leaving ActivityCancel")

	time.Sleep(2 * time.Second)

	return a + b, nil
}

func ActivitySkip(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering Activity1")
	defer log.Println("Leaving Activity1")

	time.Sleep(2 * time.Second)

	return a + b, nil
}
