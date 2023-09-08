package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/metrickeys"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/cschleiden/go-workflows/internal/workflow/cache"
	"github.com/cschleiden/go-workflows/metrics"
)

type WorkflowWorker struct {
	backend backend.Backend

	options *Options

	registry *workflow.Registry

	cache workflow.ExecutorCache

	workflowTaskQueue chan *task.Workflow

	logger *slog.Logger

	pollersWg sync.WaitGroup
	wg        sync.WaitGroup
}

func NewWorkflowWorker(backend backend.Backend, registry *workflow.Registry, options *Options) *WorkflowWorker {
	var c workflow.ExecutorCache
	if options.WorkflowExecutorCache != nil {
		c = options.WorkflowExecutorCache
	} else {
		c = cache.NewWorkflowExecutorLRUCache(backend.Metrics(), options.WorkflowExecutorCacheSize, options.WorkflowExecutorCacheTTL)
	}

	return &WorkflowWorker{
		backend: backend,

		options: options,

		registry:          registry,
		workflowTaskQueue: make(chan *task.Workflow),

		cache: c,

		logger: backend.Logger(),
	}
}

func (ww *WorkflowWorker) Start(ctx context.Context) error {
	ww.pollersWg.Add(ww.options.WorkflowPollers)

	for i := 0; i < ww.options.WorkflowPollers; i++ {
		go ww.runPoll(ctx)
	}

	go ww.runDispatcher()

	return nil
}

func (ww *WorkflowWorker) WaitForCompletion() error {
	// Wait for task pollers to finish
	ww.pollersWg.Wait()

	// Wait for tasks to finish
	ww.wg.Wait()
	close(ww.workflowTaskQueue)

	return nil
}

func (ww *WorkflowWorker) runPoll(ctx context.Context) {
	defer ww.pollersWg.Done()

	for {
		select {
		case <-ctx.Done():
			return

		default:
			task, err := ww.poll(ctx, 30*time.Second)
			if err != nil {
				ww.logger.Error("error while polling for workflow task", "error", err)
				continue
			}

			if task != nil {
				ww.wg.Add(1)
				ww.workflowTaskQueue <- task
			}
		}
	}
}

func (ww *WorkflowWorker) runDispatcher() {
	var sem chan (struct{})

	if ww.options.MaxParallelWorkflowTasks > 0 {
		sem = make(chan struct{}, ww.options.MaxParallelWorkflowTasks)
	}

	for t := range ww.workflowTaskQueue {
		if sem != nil {
			sem <- struct{}{}
		}

		t := t

		go func() {
			defer ww.wg.Done()

			// Create new context to allow workflows to complete when root context is canceled
			taskCtx := context.Background()
			ww.handle(taskCtx, t)

			if sem != nil {
				<-sem
			}
		}()
	}
}

func (ww *WorkflowWorker) handle(ctx context.Context, t *task.Workflow) {
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
	ww.backend.Metrics().Distribution(metrickeys.WorkflowTaskDelay, metrics.Tags{
		metrickeys.EventName: eventName,
	}, float64(timeInQueue/time.Millisecond))

	timer := metrics.Timer(ww.backend.Metrics(), metrickeys.WorkflowTaskProcessed, metrics.Tags{
		metrickeys.EventName: eventName,
	})

	result, err := ww.handleTask(ctx, t)
	if err != nil {
		ww.logger.ErrorContext(ctx, "could not handle workflow task", "error", err)
	}

	// Only record the time spent in the workflow code
	timer.Stop()

	state := result.State
	if state == core.WorkflowInstanceStateFinished || state == core.WorkflowInstanceStateContinuedAsNew {
		if t.WorkflowInstanceState != state {
			// If the workflow is now finished, record
			ww.backend.Metrics().Counter(metrickeys.WorkflowInstanceFinished, metrics.Tags{
				metrickeys.SubWorkflow:    fmt.Sprint(t.WorkflowInstance.SubWorkflow()),
				metrickeys.ContinuedAsNew: fmt.Sprint(state == core.WorkflowInstanceStateContinuedAsNew),
			}, 1)
		}
	}

	ww.backend.Metrics().Counter(metrickeys.ActivityTaskScheduled, metrics.Tags{}, int64(len(result.ActivityEvents)))

	if err := ww.backend.CompleteWorkflowTask(
		ctx, t, t.WorkflowInstance, state, result.Executed, result.ActivityEvents, result.TimerEvents, result.WorkflowEvents); err != nil {
		ww.logger.Error("could not complete workflow task", "error", err)
		panic("could not complete workflow task")
	}
}

func (ww *WorkflowWorker) handleTask(
	ctx context.Context,
	t *task.Workflow,
) (*workflow.ExecutionResult, error) {
	executor, err := ww.getExecutor(ctx, t)
	if err != nil {
		return nil, err
	}

	if ww.options.HeartbeatWorkflowTasks {
		// Start heartbeat while processing workflow task
		heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
		defer cancelHeartbeat()
		go ww.heartbeatTask(heartbeatCtx, t)
	}

	result, err := executor.ExecuteTask(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("executing workflow task: %w", err)
	}

	return result, nil
}

func (ww *WorkflowWorker) getExecutor(ctx context.Context, t *task.Workflow) (workflow.WorkflowExecutor, error) {
	// Try to get a cached executor
	executor, ok, err := ww.cache.Get(ctx, t.WorkflowInstance)
	if err != nil {
		ww.logger.Error("could not get cached workflow task executor", "error", err)
	}

	if !ok {
		executor, err = workflow.NewExecutor(
			ww.backend.Logger(), ww.backend.Tracer(), ww.registry, ww.backend.Converter(), ww.backend.ContextPropagators(), ww.backend, t.WorkflowInstance, t.Metadata, clock.New(),
		)
		if err != nil {
			return nil, fmt.Errorf("creating workflow task executor: %w", err)
		}
	}

	// Cache executor instance for future continuation tasks, or refresh last access time
	if err := ww.cache.Store(ctx, t.WorkflowInstance, executor); err != nil {
		ww.logger.Error("error while caching workflow task executor:", "error", err)
	}

	return executor, nil
}

func (ww *WorkflowWorker) heartbeatTask(ctx context.Context, task *task.Workflow) {
	t := time.NewTicker(ww.options.WorkflowHeartbeatInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := ww.backend.ExtendWorkflowTask(ctx, task.ID, task.WorkflowInstance); err != nil {
				ww.logger.Error("could not heartbeat workflow task", "error", err)
				panic("could not heartbeat workflow task")
			}
		}
	}
}

func (ww *WorkflowWorker) poll(ctx context.Context, timeout time.Duration) (*task.Workflow, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	task, err := ww.backend.GetWorkflowTask(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, nil
		}

		return nil, err
	}

	return task, nil
}
