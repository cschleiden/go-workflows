package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/pkg/errors"
)

type WorkflowWorker interface {
	Start(context.Context) error

	Stop() error
}

type workflowWorker struct {
	backend backend.Backend

	options *Options

	registry *workflow.Registry

	cache workflow.WorkflowExecutorCache

	workflowTaskQueue chan *task.Workflow

	logger *log.Logger

	wg *sync.WaitGroup
}

func NewWorkflowWorker(backend backend.Backend, registry *workflow.Registry, options *Options) WorkflowWorker {
	return &workflowWorker{
		backend: backend,

		options: options,

		registry:          registry,
		workflowTaskQueue: make(chan *task.Workflow),

		cache: workflow.NewWorkflowExecutorCache(workflow.DefaultWorkflowExecutorCacheOptions),

		logger: log.Default(),

		wg: &sync.WaitGroup{},
	}
}

func (ww *workflowWorker) Start(ctx context.Context) error {
	go ww.cache.StartEviction(ctx)

	for i := 0; i <= ww.options.WorkflowPollers; i++ {
		go ww.runPoll(ctx)
	}

	go ww.runDispatcher(ctx)

	return nil
}

func (ww *workflowWorker) Stop() error {
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
				log.Println("error while polling for workflow task:", err)
			} else if task != nil {
				log.Println("Got workflow task")
				ww.workflowTaskQueue <- task
			} else {
				// time.Sleep(100 * time.Millisecond)
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

			ww.wg.Add(1)
			go func() {
				defer ww.wg.Done()

				ww.handle(ctx, t)

				if sem != nil {
					log.Println("queue length", len(sem))
					<-sem
				}
			}()
		}
	}
}

func (ww *workflowWorker) handle(ctx context.Context, t *task.Workflow) {
	now := time.Now()
	log.Println("handle:", t.WorkflowInstance.GetInstanceID())
	defer log.Println("Leaving handle:", t.WorkflowInstance.GetInstanceID(), "took", time.Since(now))

	executedEvents, workflowEvents, err := ww.handleTask(ctx, t)
	if err != nil {
		ww.logger.Panic(err)
	}

	if err := ww.backend.CompleteWorkflowTask(ctx, t.WorkflowInstance, executedEvents, workflowEvents); err != nil {
		ww.logger.Panic(err)
	}
}

func (ww *workflowWorker) handleTask(ctx context.Context, t *task.Workflow) ([]history.Event, []history.WorkflowEvent, error) {
	now := time.Now()
	log.Println("handleTask:", t.WorkflowInstance.GetInstanceID())
	defer log.Println("Leaving handleTask:", t.WorkflowInstance.GetInstanceID(), "took", time.Since(now))

	executor, err := ww.getExecutor(ctx, t)
	if err != nil {
		return nil, nil, err
	}

	if ww.options.HeartbeatWorkflowTasks {
		// Start heartbeat while processing workflow task
		heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
		defer cancelHeartbeat()
		go ww.heartbeatTask(heartbeatCtx, t)
	}

	executedEvents, workflowEvents, err := executor.ExecuteTask(ctx, t)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not execute workflow task")
	}

	return executedEvents, workflowEvents, nil
}

func (ww *workflowWorker) getExecutor(ctx context.Context, t *task.Workflow) (workflow.WorkflowExecutor, error) {
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

	executor, err := workflow.NewExecutor(ww.registry, t.WorkflowInstance, clock.New())
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

func (ww *workflowWorker) poll(ctx context.Context, _ time.Duration) (*task.Workflow, error) {
	select {
	case <-ctx.Done():
		return nil, nil

	default:
		return ww.backend.GetWorkflowTask(ctx)
	}
}
