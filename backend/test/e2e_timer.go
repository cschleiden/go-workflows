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

var e2eTimerTests = []backendTest{
	{
		name: "Timer_ScheduleCancelRace",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			wf := func(ctx workflow.Context) error {
				// 1) Start of first execution slice
				tctx, cancel := workflow.WithCancel(ctx)

				// 1) Schedule timer
				x := workflow.ScheduleTimer(tctx, time.Millisecond*200)

				// 1) Force an end to the execution slice
				workflow.Sleep(ctx, time.Millisecond)

				// 2) Start of second execution slice
				// 2) Cancel timer. It should not have fired at this point,
				//    we only waited for a millisecond
				cancel()

				x.Get(ctx)

				// 2) Force the execution slice to be active for as long as it takes to fire the timer
				time.Sleep(time.Millisecond * 200)

				return nil
			}
			register(t, ctx, w, []interface{}{wf}, nil)

			_, err := runWorkflowWithResult[any](t, ctx, c, wf)
			require.NoError(t, err)
		},
	},
}
