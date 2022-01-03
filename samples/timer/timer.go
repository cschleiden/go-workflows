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
	c := client.NewClient(mb)

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
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.NewWorker(mb)

	w.RegisterWorkflow("wf1", Workflow1)

	w.RegisterActivity("a1", Activity1)

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

	a1, err := workflow.ExecuteActivity(ctx, "a1", 35, 12)
	if err != nil {
		panic("error executing activity 1")
	}

	tctx, cancel := workflow.WithCancel(ctx)

	t, err := workflow.ScheduleTimer(tctx, 5*time.Second)
	if err != nil {
		panic("could not schedule timer")
	}

	s := workflow.NewSelector()

	s.AddFuture(t, func(ctx workflow.Context, f workflow.Future) {
		log.Println("Timer fired, IsReplaying:", workflow.Replaying(ctx))
	})

	s.AddFuture(a1, func(ctx workflow.Context, f workflow.Future) {
		var r int
		f.Get(ctx, &r)
		log.Println("Activity result", r, ", IsReplaying:", workflow.Replaying(ctx))

		// Cancel timer
		cancel()
	})

	s.Select(ctx)

	return "result", nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering Activity1")

	time.Sleep(2 * time.Second)

	defer func() {
		log.Println("Leaving Activity1")
	}()

	return a + b, nil
}
