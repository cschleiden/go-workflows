package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/tester"
	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

// Test with an activity that doesn't complete quickly
func TestIncorrectWorkflowSlow(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	
	wft := tester.NewWorkflowTester[any](WFIncorrectSlow, tester.WithTestTimeout(1*time.Second))
	wft.Registry().RegisterActivity(func(ctx context.Context) error {
		// This activity will block until the test ends
		wg.Wait()
		return nil
	})
	
	// Start execution in goroutine since it might block
	done := make(chan bool)
	go func() {  
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Execution panicked: %v", r)
			}
			done <- true
		}()
		wft.Execute(context.Background())
	}()
	
	// Wait a bit then release the activity
	time.Sleep(500 * time.Millisecond)
	wg.Done()
	
	// Wait for execution to complete
	<-done
	
	require.True(t, wft.WorkflowFinished())
	_, err := wft.WorkflowResult()
	if err != nil {
		t.Logf("Got error: %v", err) 
		require.ErrorContains(t, err, "workflow completed, but there are still pending futures")
	} else {
		t.Errorf("Expected error about pending futures, but got nil")
	}
}

func WFIncorrectSlow(ctx wf.Context) error {
	wf.ExecuteActivity[any](ctx, wf.DefaultActivityOptions, "slow-activity")
	return nil // BUG: Returns without waiting for activity
}