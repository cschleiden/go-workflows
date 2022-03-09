package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/sqlite"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	b := sqlite.NewInMemoryBackend()

	// Run worker
	go RunWorker(ctx, b)

	// Start workflow via client
	c := client.New(b)

	startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2
}

func startWorkflow(ctx context.Context, c client.Client) {
	subID := uuid.NewString()

	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world", subID)
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.GetInstanceID())

	time.Sleep(2 * time.Second)
	c.SignalWorkflow(ctx, wf.GetInstanceID(), "test", 42)

	time.Sleep(2 * time.Second)
	c.SignalWorkflow(ctx, wf.GetInstanceID(), "test2", 42)

	time.Sleep(2 * time.Second)
	c.SignalWorkflow(ctx, subID, "sub-signal", 42)

	log.Println("Signaled workflow", wf.GetInstanceID())
}

func RunWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(Workflow1)
	w.RegisterWorkflow(SubWorkflow1)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}

func Workflow1(ctx workflow.Context, msg string, subID string) (string, error) {
	log.Println("Entering Workflow1")
	log.Println("\tWorkflow instance input:", msg)
	log.Println("\tIsReplaying:", workflow.Replaying(ctx))
	defer log.Println("Leaving Workflow1")

	log.Println("Waiting for first signal")
	workflow.Select(ctx,
		workflow.Receive(workflow.NewSignalChannel(ctx, "test"), func(ctx workflow.Context, c sync.Channel) {
			var r int
			c.Receive(ctx, &r)

			log.Println("Received signal:", r)
			log.Println("\tIsReplaying:", workflow.Replaying(ctx))
		}),
	)

	log.Println("Waiting for second signal")
	workflow.NewSignalChannel(ctx, "test2").Receive(ctx, nil)
	log.Println("Received second signal")

	if err := workflow.CreateSubWorkflowInstance(ctx, workflow.SubWorkflowOptions{
		InstanceID: subID,
	}, SubWorkflow1).Get(ctx, nil); err != nil {
		panic(err)
	}

	log.Println("Sub workflow finished")

	return "result", nil
}

func SubWorkflow1(ctx workflow.Context) (string, error) {
	log.Println("Waiting for signal from sub-worflow")
	defer log.Println("Leaving SubWorkflow1")

	workflow.Select(ctx,
		workflow.Receive(workflow.NewSignalChannel(ctx, "sub-signal"), func(ctx workflow.Context, c sync.Channel) {
			c.Receive(ctx, nil)
		}),
	)

	log.Println("Received.")

	return "World", nil
}
