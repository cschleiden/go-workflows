package worker

import (
	"context"
	"log"
	"time"

	"github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/pkg/errors"
)

type WorkflowWorker interface {
	Start(context.Context) error
}

type workflowWorker struct {
	backend backend.Backend

	options *Options

	registry *workflow.Registry

	cache workflow.WorkflowExecutorCache

	workflowTaskQueue chan task.Workflow

	logger *log.Logger
}

func NewWorkflowWorker(backend backend.Backend, registry *workflow.Registry, options *Options) WorkflowWorker {
	return &workflowWorker{
		backend: backend,

		options: options,

		registry:          registry,
		workflowTaskQueue: make(chan task.Workflow),

		cache: workflow.NewWorkflowExecutorCache(workflow.DefaultWorkflowExecutorCacheOptions),

		logger: log.Default(),
	}
}

func (ww *workflowWorker) Start(ctx context.Context) error {
	go ww.cache.StartEviction(ctx)

	go ww.runPoll(ctx)

	go ww.runDispatcher(ctx)

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
				log.Println("error while polling for workflow task:", err)
			} else if task != nil {
				ww.workflowTaskQueue <- *task
			}
		}
	}
}

func (ww *workflowWorker) runDispatcher(ctx context.Context) {
	var sem chan (struct{})

	if ww.options.MaxParallelWorkflowTasks > 0 {
		sem = make(chan struct{}, ww.options.MaxParallelWorkflowTasks)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ww.workflowTaskQueue:
			if sem != nil {
				sem <- struct{}{}
			}

			go func() {
				ww.handle(ctx, t)

				if sem != nil {
					<-sem
				}
			}()
		}
	}
}

func (ww *workflowWorker) handle(ctx context.Context, t task.Workflow) {
	executedEvents, workflowEvents, err := ww.handleTask(ctx, t)
	if err != nil {
		ww.logger.Panic(err)
	}

	if err := ww.backend.CompleteWorkflowTask(ctx, t.WorkflowInstance, executedEvents, workflowEvents); err != nil {
		ww.logger.Panic(err)
	}
}

func (ww *workflowWorker) handleTask(ctx context.Context, t task.Workflow) ([]history.Event, []core.WorkflowEvent, error) {
	executor, err := ww.getExecutor(ctx, t)
	if err != nil {
		return nil, nil, err
	}

	if ww.options.HeartbeatWorkflowTasks {
		// Start heartbeat while processing workflow task
		heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
		defer cancelHeartbeat()
		go ww.heartbeatTask(heartbeatCtx, &t)
	}

	executedEvents, workflowEvents, err := executor.ExecuteTask(ctx, &t)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not execute workflow task")
	}

	return executedEvents, workflowEvents, nil
}

func (ww *workflowWorker) getExecutor(ctx context.Context, t task.Workflow) (workflow.WorkflowExecutor, error) {
	if t.Kind == task.Continuation {
		executor, ok, err := ww.cache.Get(ctx, t.WorkflowInstance)
		if err != nil {
			// TODO: Can we fall back to getting a full task here?
			return nil, errors.Wrap(err, "could not get workflow task executor")
		} else if !ok {
			return nil, errors.New("workflow task executor not found in cache")
		}

		return executor, nil
	}

	executor, err := workflow.NewExecutor(ww.registry, t.WorkflowInstance)
	if err != nil {
		return nil, errors.Wrap(err, "could not create workflow executor")
	}

	// Cache executor instance for future continuation tasks
	if err := ww.cache.Store(ctx, t.WorkflowInstance, executor); err != nil {
		ww.logger.Println("error while caching workflow task executor:", err)
	}

	return executor, nil
}

func (ww *workflowWorker) heartbeatTask(ctx context.Context, task *task.Workflow) {
	t := time.NewTicker(25 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := ww.backend.ExtendWorkflowTask(ctx, task.WorkflowInstance); err != nil {
				ww.logger.Panic(err)
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
