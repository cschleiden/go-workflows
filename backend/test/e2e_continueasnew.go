package test

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var e2eContinueAsNewTests = []backendTest{
	{
		name: "ContinueAsNew/RestartsWorkflowInstance",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			wf := func(ctx workflow.Context, run int) (int, error) {
				run = run + 1
				if run < 3 {
					return run, workflow.ContinueAsNew(ctx, run)
				}

				return run, nil
			}
			register(t, ctx, w, []interface{}{wf}, nil)

			instance := runWorkflow(t, ctx, c, wf, 0)

			r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*10)
			require.NoError(t, err)
			require.Equal(t, 1, r)

			state, err := b.GetWorkflowInstanceState(ctx, instance)
			require.NoError(t, err)
			require.Equal(t, core.WorkflowInstanceStateContinuedAsNew, state)
		},
	},
	{
		name: "ContinueAsNew/Subworkflow",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			swf := func(ctx workflow.Context, run int) (int, error) {
				l := workflow.Logger(ctx)

				run = run + 1
				if run < 3 {
					l.Debug("continue as new", "run", run)
					return run, workflow.ContinueAsNew(ctx, run)
				}

				return run, nil
			}

			wf := func(ctx workflow.Context, run int) (int, error) {
				return workflow.CreateSubWorkflowInstance[int](ctx, workflow.DefaultSubWorkflowOptions, swf, run).Get(ctx)
			}
			register(t, ctx, w, []interface{}{wf, swf}, nil)

			instance := runWorkflow(t, ctx, c, wf, 0)

			r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*20)
			require.NoError(t, err)
			require.Equal(t, 3, r)
		},
	},
	{
		name:    "ContinueAsNew/OldInstancesAreRemoved",
		options: []backend.BackendOption{backend.WithRemoveContinuedAsNewInstances()},
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			var swfInstances []*workflow.Instance

			swf := func(ctx workflow.Context, iteration int) (int, error) {
				if iteration > 3 {
					return 42, nil
				}

				// Keep track of continuedasnew instances
				swfInstances = append(swfInstances, workflow.WorkflowInstance(ctx))

				return 0, workflow.ContinueAsNew(ctx, iteration+1)
			}

			swfInstanceID := uuid.NewString()

			wf := func(ctx workflow.Context) (int, error) {
				l := workflow.Logger(ctx)
				l.Debug("Starting sub workflow", "instanceID", swfInstanceID)

				r, err := workflow.CreateSubWorkflowInstance[int](ctx, workflow.SubWorkflowOptions{
					InstanceID: swfInstanceID,
				}, swf, 0).Get(ctx)

				workflow.ScheduleTimer(ctx, time.Second*2).Get(ctx)

				return r, err
			}

			register(t, ctx, w, []interface{}{wf, swf}, nil)

			wfi := runWorkflow(t, ctx, c, wf)

			// // Wait for redis to expire the keys
			// time.Sleep(autoExpirationTime * 2)

			// Main workflow should still be there
			r, err := client.GetWorkflowResult[int](ctx, c, wfi, time.Second*10)
			require.NoError(t, err)
			require.Equal(t, 42, r)

			// All continued-as-new sub-workflow instances should be expired
			for _, swfInstance := range swfInstances {
				_, err = b.GetWorkflowInstanceState(ctx, swfInstance)
				require.ErrorIs(t, err, backend.ErrInstanceNotFound)
			}
		},
	},
}
