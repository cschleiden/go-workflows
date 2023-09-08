package workflow

import (
	"log/slog"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func Test_executeActivity_ResultMismatch(t *testing.T) {
	a := func(ctx Context) (int, error) {
		return 42, nil
	}

	ctx := sync.Background()
	ctx = converter.WithConverter(ctx, converter.DefaultConverter)
	ctx = workflowstate.WithWorkflowState(
		ctx,
		workflowstate.NewWorkflowState(core.NewWorkflowInstance("a", ""), slog.Default(), clock.New()),
	)
	ctx = workflowtracer.WithWorkflowTracer(ctx, workflowtracer.New(trace.NewNoopTracerProvider().Tracer("test")))

	c := sync.NewCoroutine(ctx, func(ctx sync.Context) error {
		f := executeActivity[string](ctx, DefaultActivityOptions, 1, a)
		_, err := f.Get(ctx)
		require.Error(t, err)

		return nil
	})

	c.Execute()
	require.True(t, c.Finished())
}
func Test_executeActivity_ParamMismatch(t *testing.T) {
	a := func(ctx Context, s string, n int) (int, error) {
		return 42, nil
	}

	ctx := sync.Background()
	ctx = converter.WithConverter(ctx, converter.DefaultConverter)
	ctx = workflowstate.WithWorkflowState(
		ctx,
		workflowstate.NewWorkflowState(core.NewWorkflowInstance("a", ""), slog.Default(), clock.New()),
	)
	ctx = workflowtracer.WithWorkflowTracer(ctx, workflowtracer.New(trace.NewNoopTracerProvider().Tracer("test")))

	c := sync.NewCoroutine(ctx, func(ctx sync.Context) error {
		f := executeActivity[int](ctx, DefaultActivityOptions, 1, a)
		_, err := f.Get(ctx)
		require.Error(t, err)

		return nil
	})

	c.Execute()
	require.True(t, c.Finished())
}
