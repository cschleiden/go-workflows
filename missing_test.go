package main

import (
	"context"
	"testing"

	"github.com/cschleiden/go-workflows/tester"
	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

// Test with activity that doesn't exist in registry
func TestMissingActivity(t *testing.T) {
	wft := tester.NewWorkflowTester[any](WFMissing)
	// Don't register the activity
	
	// This should panic due to pending futures
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Got panic: %v", r)
				// Check if it's the expected panic about pending futures
				if str, ok := r.(string); ok && contains(str, "workflow completed, but there are still pending futures") {
					t.Logf("Got expected panic about pending futures")
					return
				}
			}
			// If we get here, either no panic or wrong panic
			// Try checking the workflow result for error
			if wft.WorkflowFinished() {
				_, err := wft.WorkflowResult()
				if err != nil {
					require.ErrorContains(t, err, "workflow completed, but there are still pending futures")
					return
				}
			}
			t.Errorf("Expected error about pending futures")
		}()
		
		wft.Execute(context.Background())
	}()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || (len(s) > len(substr) && contains(s[1:], substr))
}

func WFMissing(ctx wf.Context) error {
	wf.ExecuteActivity[any](ctx, wf.DefaultActivityOptions, "missing-activity")
	return nil // Returns without waiting
}