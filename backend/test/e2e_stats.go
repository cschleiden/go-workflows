package test

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

var e2eStatsTests = []backendTest{
	{
		name: "Stats_ActiveInstance",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			activityRunning := make(chan bool, 1)
			activityBlocked := make(chan bool, 1)

			a := func(ctx context.Context) error {
				activityRunning <- true
				<-activityBlocked

				return nil
			}
			wf := func(ctx workflow.Context) (bool, error) {
				workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

				workflow.NewSignalChannel[any](ctx, "test-signal").Receive(ctx)

				return true, nil
			}

			require.NoError(t, w.RegisterWorkflow(wf))
			require.NoError(t, w.RegisterActivity(a))

			s, err := b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(0), s.ActiveWorkflowInstances)
			require.Equal(t, int64(0), s.PendingTasksInQueue[core.QueueDefault].PendingWorkflowTasks)
			require.Equal(t, int64(0), s.PendingTasksInQueue[core.QueueDefault].PendingActivities)

			wfi := runWorkflow(t, ctx, c, wf)

			s, err = b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(1), s.ActiveWorkflowInstances)
			require.Equal(t, int64(1), s.PendingTasksInQueue[core.QueueDefault].PendingWorkflowTasks)
			require.Equal(t, int64(0), s.PendingTasksInQueue[core.QueueDefault].PendingActivities)

			// Start worker
			require.NoError(t, w.Start(ctx))

			// Wait until the activity is running
			<-activityRunning

			s, err = b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(0), s.PendingTasksInQueue[core.QueueDefault].PendingWorkflowTasks)
			require.Equal(t, int64(1), s.ActiveWorkflowInstances)

			s, err = b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(1), s.ActiveWorkflowInstances)
			require.Equal(t, int64(1), s.PendingTasksInQueue[core.QueueDefault].PendingActivities)

			// Let the activity finish
			activityBlocked <- true

			// Let the workflow finish
			err = c.SignalWorkflow(ctx, wfi.InstanceID, "test-signal", nil)
			require.NoError(t, err)

			err = c.WaitForWorkflowInstance(ctx, wfi, time.Second*10)
			require.NoError(t, err)

			s, err = b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(0), s.ActiveWorkflowInstances)
			require.Equal(t, int64(0), s.PendingTasksInQueue[core.QueueDefault].PendingActivities)
		},
	},
}
