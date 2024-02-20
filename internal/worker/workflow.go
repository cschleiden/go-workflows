package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metrics"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/log"
	"github.com/cschleiden/go-workflows/internal/metrickeys"
	im "github.com/cschleiden/go-workflows/internal/metrics"
	"github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/cschleiden/go-workflows/internal/workflow/cache"
	"github.com/cschleiden/go-workflows/registry"
)

type WorkflowWorkerOptions struct {
	WorkerOptions

	WorkflowExecutorCache     workflow.ExecutorCache
	WorkflowExecutorCacheSize int
	WorkflowExecutorCacheTTL  time.Duration
}

func NewWorkflowWorker(
	b backend.Backend,
	registry *registry.Registry,
	options WorkflowWorkerOptions,
) *Worker[backend.WorkflowTask, workflow.ExecutionResult] {
	if options.WorkflowExecutorCache == nil {
		options.WorkflowExecutorCache = cache.NewWorkflowExecutorLRUCache(b.Metrics(), options.WorkflowExecutorCacheSize, options.WorkflowExecutorCacheTTL)
	}

	tw := &WorkflowTaskWorker{
		backend:  b,
		registry: registry,
		cache:    options.WorkflowExecutorCache,
		logger:   b.Logger(),
	}

	return NewWorker[backend.WorkflowTask, workflow.ExecutionResult](b, tw, &options.WorkerOptions)
}

type WorkflowTaskWorker struct {
	backend  backend.Backend
	registry *registry.Registry
	cache    workflow.ExecutorCache
	logger   *slog.Logger
}

func (wtw *WorkflowTaskWorker) Start(ctx context.Context) error {
	if wtw.cache != nil {
		go wtw.cache.StartEviction(ctx)
	}

	return nil
}

// Complete implements TaskWorker.
func (wtw *WorkflowTaskWorker) Complete(ctx context.Context, result *workflow.ExecutionResult, t *backend.WorkflowTask) error {
	logger := wtw.taskLogger(t)

	state := result.State
	if state == core.WorkflowInstanceStateFinished || state == core.WorkflowInstanceStateContinuedAsNew {
		if t.WorkflowInstanceState != state {
			// If the workflow is now finished, record
			wtw.backend.Metrics().Counter(metrickeys.WorkflowInstanceFinished, metrics.Tags{
				metrickeys.SubWorkflow:    fmt.Sprint(t.WorkflowInstance.SubWorkflow()),
				metrickeys.ContinuedAsNew: fmt.Sprint(state == core.WorkflowInstanceStateContinuedAsNew),
			}, 1)
		}

		// Workflow is finished, explicitly evict from cache (if one is used)
		if wtw.cache != nil {
			if err := wtw.cache.Evict(ctx, t.WorkflowInstance); err != nil {
				logger.ErrorContext(ctx, "could not evict workflow executor from cache", "error", err)
			}
		}
	}

	wtw.backend.Metrics().Counter(metrickeys.ActivityTaskScheduled, metrics.Tags{}, int64(len(result.ActivityEvents)))

	if err := wtw.backend.CompleteWorkflowTask(
		ctx, t, t.WorkflowInstance, state, result.Executed, result.ActivityEvents, result.TimerEvents, result.WorkflowEvents); err != nil {
		logger.ErrorContext(ctx, "could not complete workflow task", "error", err)
		return fmt.Errorf("completing workflow task: %w", err)
	}

	return nil
}

func (wtw *WorkflowTaskWorker) Execute(ctx context.Context, t *backend.WorkflowTask) (*workflow.ExecutionResult, error) {
	// Record how long this task was in the queue
	firstEvent := t.NewEvents[0]
	var scheduledAt time.Time
	if firstEvent.Type == history.EventType_TimerFired {
		timerFiredAttributes := firstEvent.Attributes.(*history.TimerFiredAttributes)
		scheduledAt = timerFiredAttributes.At // Use the timestamp of the timer fired event as the schedule time
	} else {
		scheduledAt = firstEvent.Timestamp // Use the timestamp of the first event as the schedule time
	}

	eventName := fmt.Sprint(firstEvent.Type)

	timeInQueue := time.Since(scheduledAt)
	wtw.backend.Metrics().Distribution(metrickeys.WorkflowTaskDelay, metrics.Tags{
		metrickeys.EventName: eventName,
	}, float64(timeInQueue/time.Millisecond))

	timer := im.NewTimer(wtw.backend.Metrics(), metrickeys.WorkflowTaskProcessed, metrics.Tags{
		metrickeys.EventName: eventName,
	})

	executor, err := wtw.getExecutor(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("getting executor: %w", err)
	}

	result, err := executor.ExecuteTask(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("executing task: %w", err)
	}

	// Only record the time spent in the workflow code
	timer.Stop()

	return result, nil
}

func (wtw *WorkflowTaskWorker) Extend(ctx context.Context, t *backend.WorkflowTask) error {
	return wtw.backend.ExtendWorkflowTask(ctx, t.ID, t.WorkflowInstance)
}

func (wtw *WorkflowTaskWorker) Get(ctx context.Context) (*backend.WorkflowTask, error) {
	t, err := wtw.backend.GetWorkflowTask(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, nil
		}

		return nil, err
	}

	return t, nil
}

func (wtw *WorkflowTaskWorker) getExecutor(ctx context.Context, t *backend.WorkflowTask) (workflow.WorkflowExecutor, error) {
	// Try to get a cached executor
	executor, ok, err := wtw.cache.Get(ctx, t.WorkflowInstance)
	if err != nil {
		wtw.logger.ErrorContext(ctx, "could not get cached workflow task executor", "error", err)
	}

	if !ok {
		executor, err = workflow.NewExecutor(
			wtw.taskLogger(t),
			wtw.backend.Tracer(),
			wtw.registry,
			wtw.backend.Converter(),
			wtw.backend.ContextPropagators(),
			wtw.backend,
			t.WorkflowInstance,
			t.Metadata,
			clock.New(),
		)
		if err != nil {
			return nil, fmt.Errorf("creating workflow task executor: %w", err)
		}
	}

	// Cache executor instance for future continuation tasks, or refresh last access time
	if err := wtw.cache.Store(ctx, t.WorkflowInstance, executor); err != nil {
		wtw.logger.ErrorContext(ctx, "error while caching workflow task executor:", "error", err)
	}

	return executor, nil
}

func (wtw *WorkflowTaskWorker) taskLogger(t *backend.WorkflowTask) *slog.Logger {
	return wtw.logger.With(
		slog.String(log.TaskIDKey, t.ID),
		slog.String(log.InstanceIDKey, t.WorkflowInstance.InstanceID),
		slog.String(log.ExecutionIDKey, t.WorkflowInstance.ExecutionID),
	)
}
