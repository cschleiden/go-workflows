package test

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

var e2eTimerTests = []backendTest{
	{
		name: "Timer/CancelWorkflowInstance",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			a := func(ctx context.Context) error {
				return nil
			}
			wf := func(ctx workflow.Context) error {
				_, err := workflow.ScheduleTimer(ctx, time.Second*10).Get(ctx)
				if err != nil && err != workflow.Canceled {
					return err
				}

				return nil
			}
			register(t, ctx, w, []interface{}{wf}, []interface{}{a})

			instance := runWorkflow(t, ctx, c, wf)

			// Allow some time for the timer to get scheduled
			time.Sleep(time.Millisecond * 200)

			require.NoError(t, c.CancelWorkflowInstance(ctx, instance))

			_, err := client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
			require.NoError(t, err)
		},
	},
	{
		name: "Timer/CancelBeforeStarting",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			a := func(ctx context.Context) error {
				return nil
			}
			wf := func(ctx workflow.Context) error {
				tctx, cancel := workflow.WithCancel(ctx)

				f := workflow.ScheduleTimer(tctx, time.Second*10)

				// Cancel before it can be started
				cancel()

				// Force the checkpoint before continuing the execution
				workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

				_, err := f.Get(ctx)
				if err != nil && err != workflow.Canceled {
					return err
				}

				return nil
			}
			register(t, ctx, w, []interface{}{wf}, []interface{}{a})

			instance := runWorkflow(t, ctx, c, wf)
			_, err := client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
			require.NoError(t, err)

			events, err := b.GetWorkflowInstanceHistory(ctx, instance, nil)
			require.NoError(t, err)
			for _, e := range events {
				if e.Type == history.EventType_TimerScheduled {
					require.FailNow(t, "timer should not be scheduled")
				}
			}

			futureEvents, err := b.GetFutureEvents(ctx)
			require.NoError(t, err)
			require.Empty(t, futureEvents, "no future events should be scheduled")
		},
	},
	{
		name: "Timer/CancelTwice",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			a := func(ctx context.Context) error {
				return nil
			}
			wf := func(ctx workflow.Context) error {
				tctx, cancel := workflow.WithCancel(ctx)
				f := workflow.ScheduleTimer(tctx, time.Second*10)

				// Force the checkpoint before continuing the execution
				workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

				// Cancel timer
				cancel()

				// Force another checkpoint
				workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

				cancel()

				// Force another checkpoint
				workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

				if _, err := f.Get(ctx); err != nil && err != workflow.Canceled {
					return err
				}

				return nil
			}
			register(t, ctx, w, []interface{}{wf}, []interface{}{a})

			instance := runWorkflow(t, ctx, c, wf)
			_, err := client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
			require.NoError(t, err)

			historyContains(ctx, t, b, instance, history.EventType_TimerScheduled, history.EventType_TimerCanceled)

			futureEvents, err := b.GetFutureEvents(ctx)
			require.NoError(t, err)
			require.Empty(t, futureEvents, "no future events should be scheduled")
		},
	},
	{
		name: "Timer/CancelBeforeFiringRemovesFutureEvent",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			a := func(ctx context.Context) error {
				return nil
			}
			wf := func(ctx workflow.Context) error {
				tctx, cancel := workflow.WithCancel(ctx)
				f := workflow.ScheduleTimer(tctx, time.Second*10)

				// Force the checkpoint before continuing the execution
				workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

				// Cancel timer
				cancel()

				// Force another checkpoint
				workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

				if _, err := f.Get(ctx); err != nil && err != workflow.Canceled {
					return err
				}

				return nil
			}
			register(t, ctx, w, []interface{}{wf}, []interface{}{a})

			instance := runWorkflow(t, ctx, c, wf)
			_, err := client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
			require.NoError(t, err)

			historyContains(ctx, t, b, instance, history.EventType_TimerScheduled, history.EventType_TimerCanceled)

			futureEvents, err := b.GetFutureEvents(ctx)
			require.NoError(t, err)
			require.Empty(t, futureEvents, "no future events should be scheduled")
		},
	},
	{
		name: "Timer/ScheduleCancelRace",
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
