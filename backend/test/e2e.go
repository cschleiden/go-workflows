package test

import (
	"context"
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
		name           string
		workflow       interface{}
		workflowInputs []interface{}
		f              func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := setup()
			ctx := context.Background()

			c := client.New(b)
			w := worker.New(b, &worker.DefaultWorkerOptions)

			tt.f(t, ctx, c, w)

			w.Stop()

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
