package workflow

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/logger"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func Test_Cache_StoreAndGet(t *testing.T) {
	c := NewWorkflowExecutorCache(DefaultWorkflowExecutorCacheOptions)

	i := core.NewWorkflowInstance("instanceID", "executionID")

	r := NewRegistry()
	r.RegisterWorkflow(workflowWithActivity)
	e, err := NewExecutor(
		logger.NewDefaultLogger(), trace.NewNoopTracerProvider().Tracer(backend.TracerName), r, &testHistoryProvider{}, i, clock.New())
	require.NoError(t, err)

	err = c.Store(context.Background(), i, e)
	require.NoError(t, err)

	e2, ok, err := c.Get(context.Background(), i)
	require.NoError(t, err)
	require.True(t, ok)

	require.Equal(t, e, e2)
}

func Test_Cache_Evict(t *testing.T) {
	c := NewWorkflowExecutorCache(WorkflowExecutorCacheOptions{
		CacheDuration: 1, // Should evict immediately
	})

	i := core.NewWorkflowInstance("instanceID", "executionID")
	r := NewRegistry()
	r.RegisterWorkflow(workflowWithActivity)
	e, err := NewExecutor(
		logger.NewDefaultLogger(), trace.NewNoopTracerProvider().Tracer(backend.TracerName), r, &testHistoryProvider{}, i, clock.New())
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
