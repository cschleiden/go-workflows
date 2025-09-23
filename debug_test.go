package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/tester"
	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

func TestDebugFoo(t *testing.T) {
	wft := tester.NewWorkflowTester[any](WFDebug)
	
	// Register the activity
	wft.Registry().RegisterActivity(ActDebug)
	
	wft.Execute(context.Background())
	
	fmt.Printf("Workflow finished: %v\n", wft.WorkflowFinished())
	
	result, err := wft.WorkflowResult()
	fmt.Printf("Workflow result: %v, error: %v\n", result, err)
	
	if err != nil {
		require.ErrorContains(t, err, "workflow completed, but there are still pending futures")
	} else {
		t.Errorf("Expected error about pending futures, but got nil")
	}
}

func WFDebug(ctx wf.Context) error {
	fmt.Println("Workflow starting...")
	future := wf.ExecuteActivity[any](ctx, wf.DefaultActivityOptions, ActDebug)
	fmt.Printf("Activity scheduled, future: %v\n", future)
	fmt.Println("Workflow returning...")
	return nil
}

func ActDebug(context.Context) error {
	fmt.Println("Activity executing...")
	time.Sleep(10 * time.Second)
	fmt.Println("Activity completed...")
	return nil
}