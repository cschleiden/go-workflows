package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
)

var e2eStatsTests = []backendTest{
	{
		name: "Stats/ActiveInstance",
		customWorkerOptions: func(options *worker.Options) {
			options.ActivityQueues = []workflow.Queue{core.QueueDefault, "custom"}
		},
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			activityRunning := make(chan bool, 1)
			activityBlocked := make(chan bool, 1)

			a := func(ctx context.Context) error {
				activityRunning <- true
				<-activityBlocked

				return nil
			}
			a2 := func(ctx context.Context) error {
				activityRunning <- true
				<-activityBlocked

				return nil
			}
			wf := func(ctx workflow.Context) (bool, error) {
				workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

				workflow.ExecuteActivity[any](ctx, workflow.ActivityOptions{
					Queue: "custom",
				}, a2).Get(ctx)

				workflow.NewSignalChannel[any](ctx, "test-signal").Receive(ctx)

				return true, nil
			}

			require.NoError(t, w.RegisterWorkflow(wf))
			require.NoError(t, w.RegisterActivity(a))
			require.NoError(t, w.RegisterActivity(a2))

			s, err := b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(0), s.ActiveWorkflowInstances)
			require.NotContains(t, s.PendingWorkflowTasks, core.QueueDefault)

			wfi := runWorkflow(t, ctx, c, wf)

			s, err = b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(1), s.ActiveWorkflowInstances)
			require.Equal(t, int64(1), s.PendingWorkflowTasks[core.QueueDefault])
			require.Equal(t, int64(0), s.PendingActivityTasks[core.QueueDefault])

			// Start worker
			require.NoError(t, w.Start(ctx))

			// Wait until the activity is running
			<-activityRunning

			s, err = b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(0), s.PendingWorkflowTasks[core.QueueDefault])
			require.Equal(t, int64(1), s.PendingActivityTasks[core.QueueDefault])
			require.Equal(t, int64(1), s.ActiveWorkflowInstances)

			// Let the activity finish
			activityBlocked <- true

			// Wait until the activity is running
			<-activityRunning

			s, err = b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(1), s.ActiveWorkflowInstances)
			require.Equal(t, int64(0), s.PendingWorkflowTasks[core.QueueDefault])
			require.Equal(t, int64(1), s.PendingActivityTasks["custom"])

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
			require.Equal(t, int64(0), s.PendingActivityTasks[core.QueueDefault])
		},
	},
}
