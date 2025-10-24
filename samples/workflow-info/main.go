package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"

	"github.com/cschleiden/go-workflows/backend/sqlite"
)

// Workflow demonstrates accessing workflow info including history length
func Workflow(ctx workflow.Context) error {
	// Get workflow info at the start
	info := workflow.InstanceExecutionDetails(ctx)
	logger := workflow.Logger(ctx)
	logger.Info("Workflow started", "historyLength", info.HistoryLength)

	// Execute an activity
	_, err := workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, Activity).Get(ctx)
	if err != nil {
		return err
	}

	// Check history length again after activity
	info = workflow.InstanceExecutionDetails(ctx)
	logger.Info("After activity execution", "historyLength", info.HistoryLength)

	// Execute another activity
	_, err = workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, Activity).Get(ctx)
	if err != nil {
		return err
	}

	// Check history length again
	info = workflow.InstanceExecutionDetails(ctx)
	logger.Info("After second activity", "historyLength", info.HistoryLength)

	return nil
}

func Activity(ctx context.Context) error {
	log.Println("Activity executed")
	return nil
}

func main() {
	ctx := context.Background()

	// Create in-memory SQLite backend
	b := sqlite.NewInMemoryBackend()

	// Create worker
	w := worker.New(b, nil)

	// Register workflow and activity
	w.RegisterWorkflow(Workflow)
	w.RegisterActivity(Activity)

	// Start worker
	if err := w.Start(ctx); err != nil {
		panic(err)
	}

	// Create client
	c := client.New(b)

	// Create workflow instance
	wfi, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: "workflow-info-demo",
	}, Workflow)
	if err != nil {
		panic(err)
	}

	fmt.Println("Created workflow instance:", wfi.InstanceID)

	// Wait for result (10 second timeout)
	err = c.WaitForWorkflowInstance(ctx, wfi, 10*time.Second)
	if err != nil {
		panic(err)
	}

	fmt.Println("Workflow completed successfully!")
	fmt.Println("Check the logs above to see how the history length increased as the workflow executed.")
}
