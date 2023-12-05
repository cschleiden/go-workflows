package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

type CustomError struct {
	msg string
}

func (e *CustomError) Error() string {
	return e.msg
}

var e2eActivityTests = []backendTest{
	{
		name: "Activity_Panic",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			a := func(context.Context) error {
				panic("activity panic")
			}

			wf := func(ctx workflow.Context) (bool, error) {
				_, err := workflow.ExecuteActivity[int](ctx, workflow.ActivityOptions{
					RetryOptions: workflow.RetryOptions{
						MaxAttempts: 1,
					},
				}, a).Get(ctx)

				var perr *workflow.PanicError
				return errors.As(err, &perr), nil
			}
			register(t, ctx, w, []interface{}{wf}, []interface{}{a})

			output, err := runWorkflowWithResult[bool](t, ctx, c, wf)

			require.True(t, output, "error should be PanicError")
			require.NoError(t, err)
		},
	},
	{
		name: "Activity_CustomError",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			a := func(context.Context) error {
				return &CustomError{msg: "custom error"}
			}

			wf := func(ctx workflow.Context) (bool, error) {
				_, err := workflow.ExecuteActivity[int](ctx, workflow.ActivityOptions{
					RetryOptions: workflow.RetryOptions{
						MaxAttempts: 1,
					},
				}, a).Get(ctx)

				var werr *workflow.Error
				if errors.As(err, &werr) {
					return werr.Type == "CustomError" && werr.Error() == "custom error", nil
				}

				return false, nil
			}
			register(t, ctx, w, []interface{}{wf}, []interface{}{a})

			output, err := runWorkflowWithResult[bool](t, ctx, c, wf)

			require.True(t, output, "error should be PanicError")
			require.NoError(t, err)
		},
	},
	{
		name: "Activity_ExtendTask",
		customWorkerOptions: func(w *worker.Options) {
			w.ActivityHeartbeatInterval = 1 * time.Millisecond
		},
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			a := func(context.Context) error {
				time.Sleep(50 * time.Millisecond)
				return nil
			}

			wf := func(ctx workflow.Context) error {
				_, err := workflow.ExecuteActivity[int](ctx, workflow.ActivityOptions{
					RetryOptions: workflow.RetryOptions{
						MaxAttempts: 1,
					},
				}, a).Get(ctx)

				return err
			}
			register(t, ctx, w, []interface{}{wf}, []interface{}{a})

			_, err := runWorkflowWithResult[any](t, ctx, c, wf)
			require.NoError(t, err)
		},
	},
}
