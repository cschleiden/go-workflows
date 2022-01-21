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

	b := memory.NewMemoryBackend()

	// Run worker
	go RunWorker(ctx, b)

	// Start workflow via client
	c := client.New(b)

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
	w := worker.New(mb)

	w.RegisterWorkflow(Workflow1)

	w.RegisterActivity(Activity1)

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

	a1 := workflow.ExecuteActivity(ctx, Activity1, 35, 12)

	tctx, cancel := workflow.WithCancel(ctx)
	t := workflow.ScheduleTimer(tctx, 2*time.Second)
	cancel()

	workflow.NewSelector().AddFuture(t, func(ctx workflow.Context, f workflow.Future) {
		if err := f.Get(ctx, nil); err != nil {
			log.Println("Timer cancelled, IsReplaying:", workflow.Replaying(ctx))
		} else {
			log.Println("Timer fired, IsReplaying:", workflow.Replaying(ctx))
		}
	}).AddFuture(a1, func(ctx workflow.Context, f workflow.Future) {
		var r int
		if err := f.Get(ctx, &r); err != nil {
			panic(err)
		}

		log.Println("Activity result", r, ", IsReplaying:", workflow.Replaying(ctx))

		// Cancel timer
		// cancel()
	}).Select(ctx)

	return "result", nil
}

func Activity1(ctx context.Context, a, b int) (int, error) {
	log.Println("Entering Activity1")

	time.Sleep(3 * time.Second)

	defer func() {
		log.Println("Leaving Activity1")
	}()

	return a + b, nil
}
