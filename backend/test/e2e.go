package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func EndToEndBackendTest(t *testing.T, setup func() backend.Backend, teardown func(b backend.Backend)) {
	tests := []struct {
		name string
		f    func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker)
	}{
		{
			name: "SimpleWorkflow",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker) {
				wf := func(ctx workflow.Context, msg string) (string, error) {
					return msg + " world", nil
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				output, err := runWorkflowWithResult[string](t, ctx, c, wf, "hello")

				require.Equal(t, "hello world", output)
				require.NoError(t, err)
			},
		},
		{
			name: "UnregisteredWorkflow",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker) {
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
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker) {
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
			name: "UnregisteredActivity",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker) {
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
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker) {
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
			name: "SubWorkflow_PropagateCancellation",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker) {
				canceled := 0

				swf := func(ctx workflow.Context, i int) (int, error) {
					err := workflow.Sleep(ctx, time.Second*10)
					if err != nil {
						if err != workflow.Canceled {
							return 0, err
						}
					}

					if ctx.Err() != nil && ctx.Err() == workflow.Canceled {
						canceled++
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
						canceled++
					}

					return r, nil
				}
				register(t, ctx, w, []interface{}{wf, swf}, nil)

				instance := runWorkflow(t, ctx, c, wf)
				require.NoError(t, c.CancelWorkflowInstance(ctx, instance))

				r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*5)
				require.NoError(t, err)
				require.Equal(t, 6, r)

				require.Equal(t, 3, canceled)
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

			tt.f(t, ctx, c, w)

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
	return client.GetWorkflowResult[T](ctx, c, instance, time.Second*10)
}
