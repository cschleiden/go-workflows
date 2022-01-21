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

	mb := memory.NewMemoryBackend()

	// Run worker
	go RunWorker(ctx, mb)

	// Start workflow via client
	c := client.New(mb)

	startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
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
	w := worker.New(mb)

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
	workflow.NewSelector().AddChannelReceive(workflow.NewSignalChannel(ctx, "test"), func(ctx workflow.Context, c sync.Channel) {
		var r int
		c.Receive(ctx, &r)

		log.Println("Received signal:", r)
		log.Println("\tIsReplaying:", workflow.Replaying(ctx))
	}).Select(ctx)

	log.Println("Waiting for second signal")
	workflow.NewSignalChannel(ctx, "test2").Receive(ctx, nil)
	log.Println("Received second signal")

	if err := workflow.CreateSubWorkflowInstance(ctx, workflow.SubWorkflowInstanceOptions{
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

	workflow.NewSignalChannel(ctx, "sub-signal").Receive(ctx, nil)

	log.Println("Received.")

	return "World", nil
}
