package worker

import (
	"context"
	"time"

	"github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core/tasks"
)

type WorkflowWorker interface {
	Start(context.Context) error

	// Poll(ctx context.Context, timeout time.Duration) (*tasks.WorkflowTask, error)
}

type workflowWorker struct {
	backend backend.Backend

	registry *workflow.Registry

	workflowTaskQueue chan tasks.Workflow
}

func NewWorkflowWorker(backend backend.Backend, registry *workflow.Registry) WorkflowWorker {
	return &workflowWorker{
		backend: backend,

		registry:          registry,
		workflowTaskQueue: make(chan tasks.Workflow),
	}
}

func (ww *workflowWorker) Start(ctx context.Context) error {
	go ww.runPoll(ctx)

	go ww.runDispatcher(ctx)

	return nil
}

func (ww *workflowWorker) runPoll(ctx context.Context) {
	for {
		task, err := ww.poll(ctx, 30*time.Second)
		if err != nil {
			// TODO: log and ignore?
		} else if task != nil {
			ww.workflowTaskQueue <- *task
		}
	}
}

func (ww *workflowWorker) runDispatcher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-ww.workflowTaskQueue:
			go ww.handleTask(ctx, task)
		}
	}
}

func (ww *workflowWorker) handleTask(ctx context.Context, task tasks.Workflow) {
	workflowTaskExecutor := workflow.NewExecutor(ww.registry)
	commands, _ := workflowTaskExecutor.ExecuteWorkflowTask(ctx, task) // TODO: Handle error

	ww.backend.CompleteWorkflowTask(ctx, task, commands)
}

func (ww *workflowWorker) poll(ctx context.Context, timeout time.Duration) (*tasks.Workflow, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct{})

	var task *tasks.Workflow
	var err error

	go func() {
		task, err = ww.backend.GetWorkflowTask(ctx)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil, context.Canceled

	case <-done:
		return task, err
	}
}
