package cache

import (
	"context"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/contextpropagation"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/metrics"
	wf "github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func Test_Cache_StoreAndGet(t *testing.T) {
	c := NewWorkflowExecutorLRUCache(metrics.NewNoopMetricsClient(), 1, time.Second*10)

	r := wf.NewRegistry()
	r.RegisterWorkflow(workflowWithActivity)

	i := core.NewWorkflowInstance("instanceID", "executionID")
	e, err := wf.NewExecutor(
		slog.Default(), trace.NewNoopTracerProvider().Tracer(backend.TracerName), r, converter.DefaultConverter,
		[]contextpropagation.ContextPropagator{}, &testHistoryProvider{}, i, &core.WorkflowMetadata{}, clock.New(),
	)
	require.NoError(t, err)

	i2 := core.NewWorkflowInstance("instanceID2", "executionID2")
	e2, err := wf.NewExecutor(
		slog.Default(), trace.NewNoopTracerProvider().Tracer(backend.TracerName), r, converter.DefaultConverter,
		[]contextpropagation.ContextPropagator{}, &testHistoryProvider{}, i, &core.WorkflowMetadata{}, clock.New(),
	)
	require.NoError(t, err)

	err = c.Store(context.Background(), i, e)
	require.NoError(t, err)

	re, ok, err := c.Get(context.Background(), i)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, e, re)

	// Store another executor, this should evict the first one
	err = c.Store(context.Background(), i2, e2)
	require.NoError(t, err)

	re, ok, err = c.Get(context.Background(), i)
	require.NoError(t, err)
	require.False(t, ok)
}

func Test_Cache_Evict(t *testing.T) {
	c := NewWorkflowExecutorLRUCache(
		metrics.NewNoopMetricsClient(),
		128,
		1, // Should evict immediately
	)

	i := core.NewWorkflowInstance("instanceID", "executionID")
	r := wf.NewRegistry()
	r.RegisterWorkflow(workflowWithActivity)
	e, err := wf.NewExecutor(
		slog.Default(), trace.NewNoopTracerProvider().Tracer(backend.TracerName), r,
		converter.DefaultConverter, []contextpropagation.ContextPropagator{}, &testHistoryProvider{}, i,
		&core.WorkflowMetadata{}, clock.New(),
	)
	require.NoError(t, err)

	err = c.Store(context.Background(), i, e)
	require.NoError(t, err)

	go c.StartEviction(context.Background())
	time.Sleep(1 * time.Millisecond)
	runtime.Gosched()

	e2, ok, err := c.Get(context.Background(), i)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, e2)
}

func workflowWithActivity(ctx workflow.Context) (int, error) {
	r, err := workflow.ExecuteActivity[int](ctx, workflow.ActivityOptions{
		RetryOptions: workflow.RetryOptions{
			MaxAttempts: 2,
		},
	}, activity1).Get(ctx)
	if err != nil {
		return 0, err
	}

	return r, nil
}

func activity1(ctx context.Context) (int, error) {
	return 23, nil
}

type testHistoryProvider struct {
	history []*history.Event
}

func (t *testHistoryProvider) GetWorkflowInstanceHistory(ctx context.Context, instance *core.WorkflowInstance, lastSequenceID *int64) ([]*history.Event, error) {
	return t.history, nil
}
