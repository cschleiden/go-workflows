package workflow

import (
	"log/slog"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/contextvalue"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

func Test_createSubWorkflowInstance_NameAsString(t *testing.T) {
	ctx := sync.Background()
	ctx = contextvalue.WithConverter(ctx, converter.DefaultConverter)
	ctx = workflowstate.WithWorkflowState(
		ctx,
		workflowstate.NewWorkflowState(
			core.NewWorkflowInstance("a", ""), slog.Default(), noop.NewTracerProvider().Tracer("test"), clock.New()),
	)

	c := sync.NewCoroutine(ctx, func(ctx Context) error {
		createSubWorkflowInstance[int](ctx, DefaultSubWorkflowOptions, 1, "workflowName", "foo")
		return nil
	})

	c.Execute()
	require.NoError(t, c.Error())
	require.True(t, c.Finished())
}

func Test_createSubWorkflowInstance_ParamMismatch(t *testing.T) {
	wf := func(Context, int) (int, error) {
		return 42, nil
	}

	ctx := sync.Background()
	ctx = contextvalue.WithConverter(ctx, converter.DefaultConverter)
	ctx = workflowstate.WithWorkflowState(
		ctx,
		workflowstate.NewWorkflowState(
			core.NewWorkflowInstance("a", ""), slog.Default(), noop.NewTracerProvider().Tracer("test"), clock.New()),
	)

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
	ctx = contextvalue.WithConverter(ctx, converter.DefaultConverter)
	ctx = workflowstate.WithWorkflowState(
		ctx,
		workflowstate.NewWorkflowState(core.NewWorkflowInstance("a", ""), slog.Default(), noop.NewTracerProvider().Tracer("test"), clock.New()),
	)

	c := sync.NewCoroutine(ctx, func(ctx Context) error {
		f := createSubWorkflowInstance[string](ctx, DefaultSubWorkflowOptions, 1, wf)
		_, err := f.Get(ctx)
		require.Error(t, err)

		return nil
	})

	c.Execute()
	require.True(t, c.Finished())
}
