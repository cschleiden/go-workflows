package workflow

import (
	"log/slog"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/converter"
	"github.com/cschleiden/go-workflows/core"
	iconverter "github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func Test_createSubWorkflowInstance_ParamMismatch(t *testing.T) {
	wf := func(Context, int) (int, error) {
		return 42, nil
	}

	ctx := sync.Background()
	ctx = iconverter.WithConverter(ctx, converter.DefaultConverter)
	ctx = workflowstate.WithWorkflowState(
		ctx,
		workflowstate.NewWorkflowState(core.NewWorkflowInstance("a", ""), slog.Default(), clock.New()),
	)
	ctx = workflowtracer.WithWorkflowTracer(ctx, workflowtracer.New(trace.NewNoopTracerProvider().Tracer("test")))

	c := sync.NewCoroutine(ctx, func(ctx Context) error {
		f := createSubWorkflowInstance[int](ctx, DefaultSubWorkflowOptions, 1, wf, "foo")
		_, err := f.Get(ctx)
		require.Error(t, err)

		return nil
	})

	c.Execute()
	require.True(t, c.Finished())
}

func Test_createSubWorkflowInstance_ReturnMismatch(t *testing.T) {
	wf := func(ctx Context) (int, error) {
		return 42, nil
	}

	ctx := sync.Background()
	ctx = iconverter.WithConverter(ctx, converter.DefaultConverter)
	ctx = workflowstate.WithWorkflowState(
		ctx,
		workflowstate.NewWorkflowState(core.NewWorkflowInstance("a", ""), slog.Default(), clock.New()),
	)
	ctx = workflowtracer.WithWorkflowTracer(ctx, workflowtracer.New(trace.NewNoopTracerProvider().Tracer("test")))

	c := sync.NewCoroutine(ctx, func(ctx Context) error {
		f := createSubWorkflowInstance[string](ctx, DefaultSubWorkflowOptions, 1, wf)
		_, err := f.Get(ctx)
		require.Error(t, err)

		return nil
	})

	c.Execute()
	require.True(t, c.Finished())
}
