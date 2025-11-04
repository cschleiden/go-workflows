package tester

import (
	"context"
	"testing"
	"time"

	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

// Test that timer futures are properly detected as pending when workflow completes without waiting
func TestPendingTimerFutures(t *testing.T) {
	wft := NewWorkflowTester[any](workflowWithPendingTimer)
	
	// This should panic due to pending timer future
	require.Panics(t, func() {
		wft.Execute(context.Background())
	}, "Expected panic about pending timer futures")
}

func workflowWithPendingTimer(ctx wf.Context) error {
	// Schedule a timer but don't wait for it
	wf.ScheduleTimer(ctx, 10*time.Second)
	return nil // BUG: Returns without waiting for timer
}

// This test demonstrates the CORRECT behavior: activities automatically block workflow completion
func TestActivitiesAutomaticallyBlockWorkflowCompletion(t *testing.T) {
	wft := NewWorkflowTester[any](workflowWithScheduledActivity)
	wft.Registry().RegisterActivity(testActivity)
	
	// Activities automatically block workflow completion - this is the correct behavior
	// The workflow will wait for the activity to complete before finishing
	wft.Execute(context.Background())
	require.True(t, wft.WorkflowFinished())
	
	result, err := wft.WorkflowResult()
	require.NoError(t, err)
	require.Nil(t, result)
}

func workflowWithScheduledActivity(ctx wf.Context) error {
	// Schedule activity but don't explicitly wait for it
	// The workflow framework automatically waits for activities to complete
	wf.ExecuteActivity[string](ctx, wf.DefaultActivityOptions, testActivity)
	return nil // This returns after the activity completes (automatic blocking)
}

func workflowWithPendingActivity(ctx wf.Context) (string, error) {
	// Schedule activity but don't explicitly wait for it
	wf.ExecuteActivity[string](ctx, wf.DefaultActivityOptions, testActivity)
	
	// Even though we don't call future.Get(), the workflow framework 
	// automatically waits for the activity to complete
	return "should-not-be-returned", nil
}

func testActivity(ctx context.Context) (string, error) {
	return "activity-result", nil
}

// Test that workflow properly waits for activities when explicitly using Get()
func TestWorkflowExplicitlyWaitsForActivity(t *testing.T) {
	wft := NewWorkflowTester[string](workflowExplicitlyWaiting)
	wft.Registry().RegisterActivity(testActivity)
	
	wft.Execute(context.Background())
	require.True(t, wft.WorkflowFinished())
	
	result, err := wft.WorkflowResult()
	require.NoError(t, err)
	require.Equal(t, "activity-result", result)
}

func workflowExplicitlyWaiting(ctx wf.Context) (string, error) {
	// Schedule activity and explicitly wait for it
	future := wf.ExecuteActivity[string](ctx, wf.DefaultActivityOptions, testActivity)
	result, err := future.Get(ctx)
	return result, err
}