package tester

import (
	"context"
	"testing"

	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

func Test_SpanFromContext(t *testing.T) {
	var traceIDStr string

	wf := func(ctx workflow.Context) (string, error) {
		span := workflow.SpanFromContext(ctx)
		if span != nil {
			sc := span.SpanContext()
			traceIDStr = sc.TraceID().String()
		}

		return traceIDStr, nil
	}

	tester := NewWorkflowTester[string](wf)
	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	// With noop tracer, span context has zero values but the API should not panic
	_, err := tester.WorkflowResult()
	require.NoError(t, err)
}

func Test_TracerStartAndSpanContext(t *testing.T) {
	wf := func(ctx workflow.Context) error {
		tracer := workflow.Tracer(ctx)
		ctx, span := tracer.Start(ctx, "test span")

		// Should be able to get SpanContext without panicking
		sc := span.SpanContext()
		_ = sc.TraceID()
		_ = sc.SpanID()

		span.End()

		// SpanFromContext should return the span we just created
		currentSpan := workflow.SpanFromContext(ctx)
		require.NotNil(t, currentSpan)

		return nil
	}

	tester := NewWorkflowTester[any](wf)
	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	_, err := tester.WorkflowResult()
	require.NoError(t, err)
}
