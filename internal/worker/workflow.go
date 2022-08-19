package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/cschleiden/go-workflows/internal/workflow/cache"
	"github.com/cschleiden/go-workflows/log"
)

type WorkflowWorker interface {
	Start(context.Context) error

	WaitForCompletion() error
}

type workflowWorker struct {
	backend backend.Backend

	options *Options

	registry *workflow.Registry

	cache workflow.ExecutorCache

	workflowTaskQueue chan *task.Workflow

	logger log.Logger

	wg *sync.WaitGroup
}

func NewWorkflowWorker(backend backend.Backend, registry *workflow.Registry, options *Options) WorkflowWorker {
	var c workflow.ExecutorCache
	if options.WorkflowExecutorCache != nil {
		c = options.WorkflowExecutorCache
	} else {
		c = cache.NewWorkflowExecutorLRUCache(options.WorkflowExecutorCacheSize, options.WorkflowExecutorCacheTTL)
	}

	return &workflowWorker{
		backend: backend,

		options: options,

		registry:          registry,
		workflowTaskQueue: make(chan *task.Workflow),

		cache: c,

		logger: backend.Logger(),

		wg: &sync.WaitGroup{},
	}
}

func (ww *workflowWorker) Start(ctx context.Context) error {
	for i := 0; i <= ww.options.WorkflowPollers; i++ {
		go ww.runPoll(ctx)
	}

	go ww.runDispatcher()

	return nil
}

func (ww *workflowWorker) WaitForCompletion() error {
	close(ww.workflowTaskQueue)

	ww.wg.Wait()

	return nil
}

func (ww *workflowWorker) runPoll(ctx context.Context) {
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
				ww.workflowTaskQueue <- task
			}
		}
	}
}

func (ww *workflowWorker) runDispatcher() {
	var sem chan (struct{})

	if ww.options.MaxParallelWorkflowTasks > 0 {
		sem = make(chan struct{}, ww.options.MaxParallelWorkflowTasks)
	}

	for t := range ww.workflowTaskQueue {
		if sem != nil {
			sem <- struct{}{}
		}

		t := t

		ww.wg.Add(1)
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

func (ww *workflowWorker) handle(ctx context.Context, t *task.Workflow) {
	result, err := ww.handleTask(ctx, t)
	if err != nil {
		ww.logger.Panic("could not handle workflow task", "error", err)
	}

	state := core.WorkflowInstanceStateActive
	if result.Completed {
		state = core.WorkflowInstanceStateFinished
	}

	if err := ww.backend.CompleteWorkflowTask(
		ctx, t, t.WorkflowInstance, state, result.Executed, result.ActivityEvents, result.TimerEvents, result.WorkflowEvents); err != nil {
		ww.logger.Panic("could not complete workflow task", "error", err)
	}
}

func (ww *workflowWorker) handleTask(
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

func (ww *workflowWorker) getExecutor(ctx context.Context, t *task.Workflow) (workflow.WorkflowExecutor, error) {
	// Try to get a cached executor
	executor, ok, err := ww.cache.Get(ctx, t.WorkflowInstance)
	if err != nil {
		ww.logger.Error("could not get cached workflow task executor", "error", err)
	}

	if !ok {
		executor, err = workflow.NewExecutor(
			ww.backend.Logger(), ww.backend.Tracer(), ww.registry, ww.backend, t.WorkflowInstance, clock.New())
		if err != nil {
			return nil, fmt.Errorf("creating workflow executor: %w", err)
		}
	}

	// Cache executor instance for future continuation tasks, or refresh last access time
	if err := ww.cache.Store(ctx, t.WorkflowInstance, executor); err != nil {
		ww.logger.Error("error while caching workflow task executor:", "error", err)
	}

	return executor, nil
}

func (ww *workflowWorker) heartbeatTask(ctx context.Context, task *task.Workflow) {
	t := time.NewTicker(ww.options.WorkflowHeartbeatInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := ww.backend.ExtendWorkflowTask(ctx, task.ID, task.WorkflowInstance); err != nil {
				ww.logger.Panic("could not heartbeat workflow task", "error", err)
			}
		}
	}
}

func (ww *workflowWorker) poll(ctx context.Context, timeout time.Duration) (*task.Workflow, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct{})

	var task *task.Workflow
	var err error

	go func() {
		task, err = ww.backend.GetWorkflowTask(ctx)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil, nil

	case <-done:
		return task, err
	}
}
