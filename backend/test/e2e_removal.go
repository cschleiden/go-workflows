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
		name: "RemoveWorkflowInstances/Removes",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			wf := func(ctx workflow.Context) (bool, error) {
				return true, nil
			}

			register(t, ctx, w, []interface{}{wf}, nil)

			workflowA := runWorkflow(t, ctx, c, wf)
			_, err := client.GetWorkflowResult[bool](ctx, c, workflowA, time.Second*10)
			require.NoError(t, err)

			for i := 0; i < 10; i++ {
				time.Sleep(300 * time.Millisecond)

				err = b.RemoveWorkflowInstances(ctx, backend.RemoveFinishedBefore(time.Now()))
				if errors.As(err, &backend.ErrNotSupported{}) {
					t.Skip()
					return
				}

				require.NoError(t, err)

				_, err = c.GetWorkflowInstanceState(ctx, workflowA)
				if errors.Is(err, backend.ErrInstanceNotFound) {
					break
				}
			}

			require.ErrorIs(t, err, backend.ErrInstanceNotFound)
		},
	},
	{
		name: "RemoveWorkflowInstances/CleansUpHistoryAndAttributes",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			a := func(ctx context.Context) (int, error) {
				return 42, nil
			}
			wf := func(ctx workflow.Context) (int, error) {
				return workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, a).Get(ctx)
			}

			register(t, ctx, w, []interface{}{wf}, []interface{}{a})

			instance := runWorkflow(t, ctx, c, wf)
			_, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*10)
			require.NoError(t, err)

			// Verify history exists before removal
			h, err := b.GetWorkflowInstanceHistory(ctx, instance, nil)
			require.NoError(t, err)
			require.NotEmpty(t, h, "history should exist before removal")

			// Remove workflow instances
			for i := 0; i < 10; i++ {
				time.Sleep(300 * time.Millisecond)

				err = b.RemoveWorkflowInstances(ctx, backend.RemoveFinishedBefore(time.Now()))
				if errors.As(err, &backend.ErrNotSupported{}) {
					t.Skip()
					return
				}

				require.NoError(t, err)

				_, err = c.GetWorkflowInstanceState(ctx, instance)
				if errors.Is(err, backend.ErrInstanceNotFound) {
					break
				}
			}

			require.ErrorIs(t, err, backend.ErrInstanceNotFound)

			// Verify history and attributes are also cleaned up
			h, err = b.GetWorkflowInstanceHistory(ctx, instance, nil)
			require.NoError(t, err)
			require.Empty(t, h, "history and attributes should be removed along with instance")
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
			if errors.As(err, &backend.ErrNotSupported{}) {
				t.Skip()
				return
			}

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
			if errors.As(err, &backend.ErrNotSupported{}) {
				t.Skip()
				return
			}

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
