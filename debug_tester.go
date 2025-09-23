package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/tester"
	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

func TestDebugWithLogging(t *testing.T) {
	// Create a debug logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	wft := tester.NewWorkflowTester[any](WFDebug2, tester.WithLogger(logger))
	
	// Register the activity
	wft.Registry().RegisterActivity(ActDebug2)
	
	fmt.Println("=== Starting workflow execution ===")
	wft.Execute(context.Background())
	
	fmt.Printf("=== Workflow finished: %v ===\n", wft.WorkflowFinished())
	
	result, err := wft.WorkflowResult()
	fmt.Printf("=== Workflow result: %v, error: %v ===\n", result, err)
	
	if err != nil {
		fmt.Printf("Got expected error: %v\n", err)
		require.ErrorContains(t, err, "workflow completed, but there are still pending futures")
	} else {
		t.Errorf("Expected error about pending futures, but got nil")
	}
}

func WFDebug2(ctx wf.Context) error {
	fmt.Println("=== WF: Workflow starting... ===")
	future := wf.ExecuteActivity[any](ctx, wf.DefaultActivityOptions, ActDebug2)
	fmt.Printf("=== WF: Activity scheduled, future: %v ===\n", future)
	fmt.Println("=== WF: Workflow returning... ===")
	return nil
}

func ActDebug2(context.Context) error {
	fmt.Println("=== ACT: Activity executing... ===")
	time.Sleep(1 * time.Second) // Reduced sleep time
	fmt.Println("=== ACT: Activity completed... ===")
	return nil
}