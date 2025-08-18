package test

import (
	"context"
	"testing"

	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/diag"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var e2eDiagTests = []backendTest{
	{
		name: "Diag/Paging",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			diagBackend, ok := b.(diag.Backend)
			if !ok {
				t.Skip("Backend does not implement diag.Backend")
			}

			wf := func(ctx workflow.Context) (bool, error) {
				return true, nil
			}

			register(t, ctx, w, []any{wf}, nil)

			for i := 0; i < 50; i++ {
				runWorkflow(t, ctx, c, wf)
			}

			afterInstanceID := ""
			afterExecutionID := ""

			// Fetch 5 pages
			for i := 0; i < 5; i++ {
				refs, err := diagBackend.GetWorkflowInstances(ctx, afterInstanceID, afterExecutionID, 10)
				require.NoError(t, err)
				require.Len(t, refs, 10)

				require.NotEqual(t, afterInstanceID, refs[len(refs)-1].Instance.InstanceID)

				afterInstanceID = refs[len(refs)-1].Instance.InstanceID
				afterExecutionID = refs[len(refs)-1].Instance.ExecutionID
			}

			refs, err := diagBackend.GetWorkflowInstances(ctx, afterInstanceID, afterExecutionID, 10)
			require.NoError(t, err)
			require.Len(t, refs, 0)
		},
	},
	{
		name: "Diag/GetWorkflowInstance_ReturnsQueue",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			diagBackend, ok := b.(diag.Backend)
			if !ok {
				t.Skip("Backend does not implement diag.Backend")
			}

			wf := func(ctx workflow.Context) (bool, error) {
				return true, nil
			}

			register(t, ctx, w, []any{wf}, nil)

			wfi := runWorkflow(t, ctx, c, wf)

			wfi2, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
				InstanceID: uuid.NewString(),
				Queue:      "custom-queue",
			}, wf)
			require.NoError(t, err)

			wfState, err := diagBackend.GetWorkflowInstance(ctx, wfi)
			require.NoError(t, err)
			require.NotNil(t, wfState)
			require.Equal(t, string(workflow.QueueDefault), wfState.Queue)

			wfState2, err := diagBackend.GetWorkflowInstance(ctx, wfi2)
			require.NoError(t, err)
			require.NotNil(t, wfState2)
			require.Equal(t, "custom-queue", wfState2.Queue)
		},
	},
}
