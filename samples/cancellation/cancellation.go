package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/cschleiden/go-dt/internal/sync"
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

	time.Sleep(3 * time.Second)
	c.SignalWorkflow(ctx, wf, "cancelled", nil)
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.NewWorker(mb)

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

	cancelChannel := workflow.NewSignalChannel(ctx, "cancelled")

	canceled := false
	for i := 0; i < 2 && !canceled; i++ {
		a, err := workflow.ExecuteActivity(ctx, Activity1, 1, 2)
		if err != nil {
			panic(err)
		}

		workflow.NewSelector().
			AddChannelReceive(cancelChannel, func(ctx sync.Context, c sync.Channel) {
				log.Println("cancelled")
				canceled = true
			}).
			AddFuture(a, func(ctx sync.Context, f sync.Future) {
				var r int
				err := f.Get(ctx, &r)
				if err != nil {
					panic(err)
				}

				log.Println("\tActivity result:", r)
			}).
			Select(ctx)
	}

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
