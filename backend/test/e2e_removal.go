package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

var e2eRemovalTests = []backendTest{
	{
		name: "RemoveWorkflowInstances_Removes",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			wf := func(ctx workflow.Context) (bool, error) {
				return true, nil
			}

			register(t, ctx, w, []interface{}{wf}, nil)

			workflowA := runWorkflow(t, ctx, c, wf)
			_, err := client.GetWorkflowResult[bool](ctx, c, workflowA, time.Second*10)
			require.NoError(t, err)

			now := time.Now()
			time.Sleep(300 * time.Millisecond)

			_, err = runWorkflowWithResult[bool](t, ctx, c, wf)
			require.NoError(t, err)

			err = b.RemoveWorkflowInstances(ctx, backend.RemoveFinishedBefore(now))
			if errors.As(err, &backend.ErrNotSupported{}) {
				t.Skip()
				return
			}

			require.NoError(t, err)

			_, err = c.GetWorkflowInstanceState(ctx, workflowA)
			require.ErrorIs(t, err, backend.ErrInstanceNotFound)
		},
	},
	{
		name: "AutoExpiration/StartsWorkflowAndRemoves",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			wf := func(ctx workflow.Context) (bool, error) {
				return true, nil
			}

			register(t, ctx, w, []interface{}{wf}, nil)

			err := c.StartAutoExpiration(ctx, time.Millisecond)
			require.NoError(t, err)

			workflowA := runWorkflow(t, ctx, c, wf)
			_, err = client.GetWorkflowResult[bool](ctx, c, workflowA, time.Second*10)
			require.NoError(t, err)

			for i := 0; i < 10; i++ {
				time.Sleep(100 * time.Millisecond)
				_, err = c.GetWorkflowInstanceState(ctx, workflowA)
				if err != backend.ErrInstanceNotFound {
					continue
				} else {
					break
				}
			}

			require.ErrorIs(t, err, backend.ErrInstanceNotFound)
		},
	},
	{
		name: "AutoExpiration/UpdateDelay",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			wf := func(ctx workflow.Context) (bool, error) {
				return true, nil
			}

			register(t, ctx, w, []interface{}{wf}, nil)

			err := c.StartAutoExpiration(ctx, time.Hour)
			require.NoError(t, err)

			workflowA := runWorkflow(t, ctx, c, wf)
			_, err = client.GetWorkflowResult[bool](ctx, c, workflowA, time.Second*10)
			require.NoError(t, err)

			time.Sleep(1000 * time.Millisecond)
			_, err = c.GetWorkflowInstanceState(ctx, workflowA)
			require.NotErrorIs(t, err, backend.ErrInstanceNotFound)

			// update delay
			err = c.StartAutoExpiration(ctx, time.Millisecond)
			require.NoError(t, err)

			for i := 0; i < 10; i++ {
				time.Sleep(100 * time.Millisecond)
				_, err = c.GetWorkflowInstanceState(ctx, workflowA)
				if err != backend.ErrInstanceNotFound {
					continue
				} else {
					break
				}
			}

			require.ErrorIs(t, err, backend.ErrInstanceNotFound)
		},
	},
}
