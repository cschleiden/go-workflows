package main

import (
	"context"
	"log"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/samples"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"

	"github.com/google/uuid"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend := samples.GetBackend("orchestrator", backend.WithWorkerName("orchestrator-worker"))

	orchestrator := worker.NewWorkflowOrchestrator(
		backend,
		nil,
	)

	if err := orchestrator.Start(ctx); err != nil {
		panic("could not start orchestrator: " + err.Error())
	}

	// Create instance ID
	instanceID := uuid.NewString()

	// Create and run workflow using the orchestrator - no explicit registration needed
	instance, err := orchestrator.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: instanceID,
	}, SimpleWorkflow, "Hello from orchestrator!")
	if err != nil {
		panic("could not create workflow instance: " + err.Error())
	}

	// Wait for result using client package directly
	result, err := client.GetWorkflowResult[string](ctx, orchestrator.Client, instance, 10*time.Second)
	if err != nil {
		panic("error getting workflow result: " + err.Error())
	}

	log.Printf("Workflow result: %s\n", result)

	// Clean shutdown
	cancel()

	if err := orchestrator.WaitForCompletion(); err != nil {
		panic("could not stop orchestrator: " + err.Error())
	}
}

// SimpleWorkflow is a basic workflow that calls an activity and returns its result
func SimpleWorkflow(ctx workflow.Context, message string) (string, error) {
	f := workflow.ExecuteActivity[string](ctx, workflow.DefaultActivityOptions, ProcessMessage, message)

	result, err := f.Get(ctx)
	if err != nil {
		return "", err
	}

	return result, nil
}

// ProcessMessage is a simple activity that processes a message
// When orchestrator mode is enabled, this will be automatically registered
func ProcessMessage(ctx context.Context, message string) (string, error) {
	return message + " (processed by activity)", nil
}
