package test

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

var e2eStatsTests = []backendTest{
	{
		name: "Stats_ActiveInstance",
		f: func(t *testing.T, ctx context.Context, c client.Client, w *worker.Worker, b TestBackend) {
			as := make(chan bool, 1)
			af := make(chan bool, 1)

			a := func(ctx context.Context) error {
				as <- true
				<-af

				return nil
			}
			wf := func(ctx workflow.Context) (bool, error) {
				workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

				workflow.NewSignalChannel[any](ctx, "test-signal").Receive(ctx)

				return true, nil
			}
			register(t, ctx, w, []interface{}{wf}, []interface{}{a})

			s, err := b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(0), s.ActiveWorkflowInstances)
			require.Equal(t, int64(0), s.PendingActivities)

			wfi := runWorkflow(t, ctx, c, wf)

			s, err = b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(1), s.ActiveWorkflowInstances)

			<-as

			s, err = b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(1), s.ActiveWorkflowInstances)
			require.Equal(t, int64(1), s.PendingActivities)

			af <- true

			err = c.SignalWorkflow(ctx, wfi.InstanceID, "test-signal", nil)
			require.NoError(t, err)

			err = c.WaitForWorkflowInstance(ctx, wfi, time.Second*10)
			require.NoError(t, err)

			s, err = b.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, int64(0), s.ActiveWorkflowInstances)
			require.Equal(t, int64(0), s.PendingActivities)
		},
	},
}
