package workflow

import (
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/logger"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func subWorkflow(ctx Context) (string, error) {
	return "test", nil
}

func Test_createSubWorkflowInstance_checksReturnType(t *testing.T) {
	ctx := testContext()

	var err error
	cr := sync.NewCoroutine(ctx, func(ctx sync.Context) error {
		f := createSubWorkflowInstance[int](ctx, DefaultSubWorkflowOptions, 0, subWorkflow)
		_, err = f.Get(ctx)

		return nil
	})
	cr.Execute()

	require.Error(t, err)
	require.EqualError(t, err, "checking subworkflow result: expected return value int, but got string")
}

func testContext() Context {
	ctx := sync.Background()

	instance := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	wfState := workflowstate.NewWorkflowState(instance, logger.NewDefaultLogger(), clock.New())
	ctx = workflowstate.WithWorkflowState(ctx, wfState)

	wfTracer := workflowtracer.New(trace.NewNoopTracerProvider().Tracer("test"))
	ctx = workflowtracer.WithWorkflowTracer(ctx, wfTracer)

	return ctx
}
