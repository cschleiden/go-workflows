package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/samples"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	// b := sqlite.NewInMemoryBackend()
	b, err := redis.NewRedisBackend("localhost:6379", "", "RedisPassw0rd", 0)
	if err != nil {
		panic(err)
	}

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
	samples.Trace(ctx, "Entering Workflow1")

	samples.Trace(ctx, "Waiting for first signal")
	workflow.Select(ctx,
		workflow.Receive(workflow.NewSignalChannel[int](ctx, "test"), func(ctx workflow.Context, r int, ok bool) {
			samples.Trace(ctx, "Received signal:", r)
		}),
	)

	samples.Trace(ctx, "Waiting for second signal")
	workflow.NewSignalChannel[int](ctx, "test2").Receive(ctx)
	samples.Trace(ctx, "Received second signal")

	if _, err := workflow.CreateSubWorkflowInstance[any](ctx, workflow.SubWorkflowOptions{
		InstanceID: subID,
	}, SubWorkflow1).Get(ctx); err != nil {
		panic(err)
	}

	samples.Trace(ctx, "Sub workflow finished")

	return "result", nil
}

func SubWorkflow1(ctx workflow.Context) (string, error) {
	samples.Trace(ctx, "Waiting for signal from sub-worflow")

	c := workflow.NewSignalChannel[int](ctx, "sub-signal")
	c.Receive(ctx)

	samples.Trace(ctx, "Received sub-workflow signal")

	return "World", nil
}
