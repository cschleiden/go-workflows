package test

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func EndToEndBackendTest(t *testing.T, setup func() backend.Backend, teardown func(b backend.Backend)) {
	tests := []struct {
		name string
		f    func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b backend.Backend)
	}{
		{
			name: "SimpleWorkflow",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b backend.Backend) {
				wf := func(ctx workflow.Context, msg string) (string, error) {
					return msg + " world", nil
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				output, err := runWorkflowWithResult[string](t, ctx, c, wf, "hello")

				require.NoError(t, err)
				require.Equal(t, "hello world", output)
			},
		},
		{
			name: "UnregisteredWorkflow_Errors",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b backend.Backend) {
				wf := func(ctx workflow.Context, msg string) (string, error) {
					return msg + " world", nil
				}
				register(t, ctx, w, nil, nil)

				output, err := runWorkflowWithResult[string](t, ctx, c, wf, "hello")

				require.Zero(t, output)
				require.ErrorContains(t, err, "workflow 1 not found")
			},
		},
		{
			name: "WorkflowArgumentMismatch",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b backend.Backend) {
				wf := func(ctx workflow.Context, p1 int) (int, error) {
					return 42, nil
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				output, err := runWorkflowWithResult[int](t, ctx, c, wf)

				require.Zero(t, output)
				require.ErrorContains(t, err, "converting workflow inputs: mismatched argument count: expected 1, got 0")
			},
		},
		{
			name: "UnregisteredActivity_Errors",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b backend.Backend) {
				a := func(context.Context) error { return nil }
				wf := func(ctx workflow.Context) (int, error) {
					return workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, a).Get(ctx)
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				output, err := runWorkflowWithResult[int](t, ctx, c, wf)

				require.Zero(t, output)
				require.ErrorContains(t, err, "activity not found")
			},
		},
		{
			name: "ActivityArgumentMismatch",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b backend.Backend) {
				a := func(context.Context, int, int) error { return nil }
				wf := func(ctx workflow.Context) (int, error) {
					return workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, a, 42).Get(ctx)
				}
				register(t, ctx, w, []interface{}{wf}, []interface{}{a})

				output, err := runWorkflowWithResult[int](t, ctx, c, wf)

				require.Zero(t, output)
				require.ErrorContains(t, err, "converting activity inputs: mismatched argument count: expected 2, got 1")
			},
		},
		{
			name: "SubWorkflow_Simple",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b backend.Backend) {
				swf := func(ctx workflow.Context, i int) (int, error) {
					return i * 2, nil
				}
				wf := func(ctx workflow.Context) (int, error) {
					r, err := workflow.CreateSubWorkflowInstance[int](ctx, workflow.DefaultSubWorkflowOptions, swf, 1).Get(ctx)
					if err != nil {
						return 0, err
					}

					return r, nil
				}
				register(t, ctx, w, []interface{}{wf, swf}, nil)

				instance := runWorkflow(t, ctx, c, wf)

				r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*500)
				require.NoError(t, err)
				require.Equal(t, 2, r)
			},
		},
		{
			name: "SubWorkflow_PropagateCancellation",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b backend.Backend) {
				canceled := int32(0)

				swf := func(ctx workflow.Context, i int) (int, error) {
					err := workflow.Sleep(ctx, time.Second*10)
					if err != nil && err != workflow.Canceled {
						return 0, err
					}

					if ctx.Err() != nil && ctx.Err() == workflow.Canceled {
						atomic.AddInt32(&canceled, 1)
					}

					return i * 2, nil
				}
				wf := func(ctx workflow.Context) (int, error) {
					swfs := make([]workflow.Future[int], 0)

					swfs = append(swfs, workflow.CreateSubWorkflowInstance[int](ctx, workflow.DefaultSubWorkflowOptions, swf, 1))
					swfs = append(swfs, workflow.CreateSubWorkflowInstance[int](ctx, workflow.DefaultSubWorkflowOptions, swf, 2))

					r := 0

					for _, f := range swfs {
						sr, err := f.Get(ctx)
						if err != nil && err != workflow.Canceled {
							return 0, err
						}

						r = r + sr
					}

					if ctx.Err() != nil && ctx.Err() == workflow.Canceled {
						atomic.AddInt32(&canceled, 1)
					}

					return r, nil
				}
				register(t, ctx, w, []interface{}{wf, swf}, nil)

				instance := runWorkflow(t, ctx, c, wf)
				require.NoError(t, c.CancelWorkflowInstance(ctx, instance))

				r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*500)
				require.NoError(t, err)
				require.Equal(t, int32(3), canceled)
				require.Equal(t, 6, r)
			},
		},
		{
			name: "SubWorkflow_CancelBeforeStarting",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b backend.Backend) {
				swInstanceID := "subworkflow"

				swfrun := 0
				swf := func(ctx workflow.Context, i int) (int, error) {
					swfrun++
					return i * 2, nil
				}
				wf := func(ctx workflow.Context) (int, error) {
					swfctx, cancel := workflow.WithCancel(ctx)

					f := workflow.CreateSubWorkflowInstance[int](swfctx, workflow.SubWorkflowOptions{
						InstanceID: swInstanceID,
					}, swf, 1)

					// Cancel before it can be started
					cancel()

					// Force the checkpoint before continuing the execution
					workflow.Sleep(ctx, time.Millisecond*2)

					r, err := f.Get(ctx)
					if err != nil && err != workflow.Canceled {
						return 0, err
					}

					return r, nil
				}
				register(t, ctx, w, []interface{}{wf, swf}, nil)

				instance := runWorkflow(t, ctx, c, wf)
				r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*5)
				require.NoError(t, err)
				require.Equal(t, 0, r)
				require.Equal(t, 0, swfrun, "sub-workflow should not run")

				_, err = b.GetWorkflowInstanceState(ctx, &core.WorkflowInstance{
					InstanceID: swInstanceID,
				})

				require.Error(t, err)
				require.Equal(t, backend.ErrInstanceNotFound, err)
			},
		},
		{
			name: "Timer_Cancel",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b backend.Backend) {
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
			name: "Timer_CancelBeforeStarting",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b backend.Backend) {
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
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := setup()
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)

			c := client.New(b)
			w := worker.New(b, &worker.DefaultWorkerOptions)

			tt.f(t, ctx, c, w, b)

			cancel()
			if err := w.WaitForCompletion(); err != nil {
				fmt.Println("Worker did not stop in time")
				t.FailNow()
			}

			if teardown != nil {
				teardown(b)
			}
		})
	}
}

func register(t *testing.T, ctx context.Context, w worker.Worker, workflows []interface{}, activities []interface{}) {
	for _, wf := range workflows {
		require.NoError(t, w.RegisterWorkflow(wf))
	}

	for _, a := range activities {
		require.NoError(t, w.RegisterActivity(a))
	}

	err := w.Start(ctx)
	require.NoError(t, err)
}

func runWorkflow(t *testing.T, ctx context.Context, c client.Client, wf interface{}, inputs ...interface{}) *workflow.Instance {
	instance, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, wf, inputs...)
	require.NoError(t, err)

	return instance
}

func runWorkflowWithResult[T any](t *testing.T, ctx context.Context, c client.Client, wf interface{}, inputs ...interface{}) (T, error) {
	instance := runWorkflow(t, ctx, c, wf, inputs...)

	log.Println("Workflow instance:", instance.InstanceID)

	return client.GetWorkflowResult[T](ctx, c, instance, time.Second*10)
}
