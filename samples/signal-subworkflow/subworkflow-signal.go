package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/google/uuid"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/samples"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
)

func main() {
	ctx := context.Background()

	b := samples.GetBackend("subworkflow-signal")

	// Run worker
	go RunWorker(ctx, b)

	// Start workflow via client
	c := client.New(b)

	startWorkflow(ctx, c)

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2
}

func startWorkflow(ctx context.Context, c *client.Client) {
	subID := uuid.NewString()

	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "Hello world", subID)
	if err != nil {
		panic("could not start workflow")
	}

	log.Println("Started workflow", wf.InstanceID)
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
	logger := workflow.Logger(ctx)
	logger.Debug("Entering Workflow1")

	logger.Debug("Scheduling sub-workflow")
	fsw := workflow.CreateSubWorkflowInstance[string](ctx, workflow.SubWorkflowOptions{
		InstanceID: subID,
	}, SubWorkflow1)

	if _, err := workflow.SignalWorkflow(ctx, subID, "sub-signal", 42).Get(ctx); err != nil {
		return "", fmt.Errorf("could not signal sub-workflow: %w", err)
	}

	r, err := fsw.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("could not get sub-workflow result: %w", err)
	}

	return r, nil
}

func SubWorkflow1(ctx workflow.Context) (string, error) {
	logger := workflow.Logger(ctx)
	logger.Debug("Waiting for signal in sub-workflow")

	c := workflow.NewSignalChannel[int](ctx, "sub-signal")
	c.Receive(ctx)

	logger.Debug("Received sub-workflow signal")

	return "World", nil
}
