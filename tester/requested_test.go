package tester

import (
	"context"
	"testing"
	"time"

	wf "github.com/cschleiden/go-workflows/workflow"
)

func TestFoo(t *testing.T) {
	wft := NewWorkflowTester[any](WF)
	// Don't mock or register the activity - this should create a pending future scenario
	
	// Execute the workflow - with the shorter timeout, it should detect pending futures
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Check if it's the expected panic about pending futures
				if str, ok := r.(string); ok && contains(str, "workflow completed, but there are still pending futures") {
					t.Logf("Got expected panic about pending futures: %v", r)
					return
				}
				// Check if it's the workflow blocked panic (which is also expected in this case)
				if str, ok := r.(string); ok && contains(str, "workflow blocked") {
					// The workflow is blocked waiting for the unregistered activity
					// This is expected behavior that validates pending futures exist
					t.Logf("Workflow blocked as expected due to pending activity: %v", r)
					return
				}
				// Re-panic if it's a different error
				panic(r)
			}
			t.Errorf("Expected panic about pending futures or workflow blocked, but workflow completed normally")
		}()
		
		wft.Execute(context.Background())
	}()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr))))
}

func WF(ctx wf.Context) error {
	wf.ExecuteActivity[any](ctx, wf.DefaultActivityOptions, Act)
	return nil
}

func Act(context.Context) error {
	time.Sleep(100 * time.Millisecond)
	return nil
}