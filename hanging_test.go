package main

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/tester"
	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

// Test with activity that never completes
func TestHangingActivity(t *testing.T) {
	wft := tester.NewWorkflowTester[any](WFHanging, tester.WithTestTimeout(100*time.Millisecond))
	
	// Register an activity that blocks forever
	wft.Registry().RegisterActivity(func(ctx context.Context) error {
		// Block until context is cancelled 
		<-ctx.Done()
		return ctx.Err()
	})
	
	// This should timeout or panic due to pending futures
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Got expected panic: %v", r)
				require.Contains(t, r.(string), "workflow completed, but there are still pending futures")
				return
			}
			t.Errorf("Expected panic about pending futures")
		}()
		
		wft.Execute(context.Background())
	}()
}

func WFHanging(ctx wf.Context) error {
	wf.ExecuteActivity[any](ctx, wf.DefaultActivityOptions, "hanging-activity")
	return nil // Returns without waiting
}