package redis

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/diag"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Test for getting an active workflow instance in diag UI
func Test_Diag_GetActiveWorkflowInstance(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	rclient := getClient()
	setup := getCreateBackend(rclient)

	b := setup()

	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	bd := b.(diag.Backend)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register a workflow that sleeps for a long time
	w := worker.New(b, nil)
	longRunningWf := func(ctx workflow.Context) error {
		return workflow.Sleep(ctx, time.Hour)
	}
	w.RegisterWorkflow(longRunningWf)

	if err := w.Start(ctx); err != nil {
		t.Fatal("could not start worker:", err)
	}
	t.Cleanup(func() {
		cancel()
		w.WaitForCompletion()
	})

	c := client.New(b)

	// Create a long-running workflow instance
	wfi, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, longRunningWf)
	require.NoError(t, err)

	// Wait a bit for the workflow to start
	time.Sleep(500 * time.Millisecond)

	// Try to get the active workflow instance - this should not fail
	instanceRef, err := bd.GetWorkflowInstance(ctx, wfi)
	require.NoError(t, err, "GetWorkflowInstance should not fail for active workflow")
	require.NotNil(t, instanceRef)
	require.Equal(t, wfi.InstanceID, instanceRef.Instance.InstanceID)
	require.Equal(t, wfi.ExecutionID, instanceRef.Instance.ExecutionID)

	// Try to get the workflow history - this is what the diag UI calls
	history, err := b.GetWorkflowInstanceHistory(ctx, wfi, nil)
	require.NoError(t, err, "GetWorkflowInstanceHistory should not fail for active workflow")
	require.NotEmpty(t, history, "Active workflow should have history")

	// Cancel the workflow
	err = c.CancelWorkflowInstance(ctx, wfi)
	require.NoError(t, err)

	// Wait for it to finish
	time.Sleep(500 * time.Millisecond)

	// Now verify it works for completed workflow too
	instanceRef2, err := bd.GetWorkflowInstance(ctx, wfi)
	require.NoError(t, err)
	require.NotNil(t, instanceRef2)

	history2, err := b.GetWorkflowInstanceHistory(ctx, wfi, nil)
	require.NoError(t, err)
	require.NotEmpty(t, history2)
}
