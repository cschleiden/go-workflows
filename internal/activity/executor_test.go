package activity

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestExecutor_ExecuteActivity(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(t *testing.T, r *workflow.Registry) *history.ActivityScheduledAttributes
		result func(t *testing.T, result payload.Payload, err error)
	}{
		{
			name: "unknown activity",
			setup: func(t *testing.T, r *workflow.Registry) *history.ActivityScheduledAttributes {
				return &history.ActivityScheduledAttributes{
					Name: "unknown",
				}
			},
			result: func(t *testing.T, result payload.Payload, err error) {
				require.Nil(t, result)
				require.Error(t, err)
				require.EqualError(t, err, "activity not found")
			},
		},
		{
			name: "mismatched argument count",
			setup: func(t *testing.T, r *workflow.Registry) *history.ActivityScheduledAttributes {
				a := func(context.Context, int, int) error { return nil }
				require.NoError(t, r.RegisterActivity(a))

				return &history.ActivityScheduledAttributes{
					Name: fn.Name(a),
				}
			},
			result: func(t *testing.T, result payload.Payload, err error) {
				require.Nil(t, result)
				require.Error(t, err)
				require.EqualError(t, err, "converting activity inputs: mismatched argument count: expected 2, got 0")
			},
		},
		{
			name: "wrap error",
			setup: func(t *testing.T, r *workflow.Registry) *history.ActivityScheduledAttributes {
				a := func(context.Context, int) error {
					return errors.New("some error")
				}
				require.NoError(t, r.RegisterActivity(a))

				inputs, _ := args.ArgsToInputs(converter.DefaultConverter, 42)

				return &history.ActivityScheduledAttributes{
					Name:   fn.Name(a),
					Inputs: inputs,
				}
			},
			result: func(t *testing.T, result payload.Payload, err error) {
				require.Nil(t, result)
				require.Error(t, err)

				var expectedErr *workflowerrors.Error
				require.ErrorAs(t, err, &expectedErr)
			},
		},
		{
			name: "handle panic",
			setup: func(t *testing.T, r *workflow.Registry) *history.ActivityScheduledAttributes {
				a := func(context.Context, int) error {
					panic("activity panic")
				}
				require.NoError(t, r.RegisterActivity(a))

				inputs, _ := args.ArgsToInputs(converter.DefaultConverter, 42)

				return &history.ActivityScheduledAttributes{
					Name:   fn.Name(a),
					Inputs: inputs,
				}
			},
			result: func(t *testing.T, result payload.Payload, err error) {
				require.Nil(t, result)
				require.Error(t, err)

				var expectedErr *workflowerrors.Error
				require.ErrorAs(t, err, &expectedErr)
				e := err.(*workflowerrors.Error)
				require.Equal(t, e.Type, "PanicError")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := workflow.NewRegistry()
			attr := tt.setup(t, r)

			e := &Executor{
				logger:    slog.Default(),
				r:         r,
				converter: converter.DefaultConverter,
				tracer:    trace.NewNoopTracerProvider().Tracer(""),
			}
			got, err := e.ExecuteActivity(context.Background(), &task.Activity{
				ID:               uuid.NewString(),
				WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
				Event:            history.NewHistoryEvent(1, time.Now(), history.EventType_ActivityScheduled, attr),
			})
			tt.result(t, got, err)
		})
	}
}
