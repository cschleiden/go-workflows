package worker

import (
	"context"
	"time"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
)

type WorkflowWorker interface {
	Start(context.Context) error

	// Poll(ctx context.Context, timeout time.Duration) (*task.WorkflowTask, error)
}

type workflowWorker struct {
	backend backend.Backend

	registry *workflow.Registry

	workflowTaskQueue chan task.Workflow
}

func NewWorkflowWorker(backend backend.Backend, registry *workflow.Registry) WorkflowWorker {
	return &workflowWorker{
		backend: backend,

		registry:          registry,
		workflowTaskQueue: make(chan task.Workflow),
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

func (ww *workflowWorker) handleTask(ctx context.Context, task task.Workflow) {
	workflowTaskExecutor := workflow.NewExecutor(ww.registry, &task)
	commands, _ := workflowTaskExecutor.ExecuteWorkflowTask(ctx) // TODO: Handle error

	newEvents := make([]history.HistoryEvent, 0)

	for _, c := range commands {
		switch c.Type {
		case command.CommandType_ScheduleActivityTask:
			a := c.Attr.(command.ScheduleActivityTaskCommandAttr)

			newEvents = append(newEvents, history.NewHistoryEvent(
				history.HistoryEventType_ActivityScheduled,
				c.ID,
				history.ActivityScheduledAttributes{
					Name:    a.Name,
					Version: a.Version,
					Inputs:  a.Inputs,
				},
			))

		case command.CommandType_ScheduleTimer:
			a := c.Attr.(command.ScheduleTimerCommandAttr)

			timerEvent := history.NewHistoryEvent(
				history.HistoryEventType_TimerScheduled,
				c.ID,
				history.TimerScheduledAttributes{
					At: a.At,
				},
			)
			timerEvent.VisibleAt = &a.At

			newEvents = append(newEvents, timerEvent)

		case command.CommandType_CompleteWorkflow:
			a := c.Attr.(command.CompleteWorkflowCommandAttr)

			newEvents = append(newEvents, history.NewHistoryEvent(
				history.HistoryEventType_WorkflowExecutionFinished,
				c.ID,
				history.ExecutionCompletedAttributes{
					Result: a.Result,
				},
			))

		default:
			// panic("unsupported command")
		}
	}

	ww.backend.CompleteWorkflowTask(ctx, task, newEvents)
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
		return nil, context.Canceled

	case <-done:
		return task, err
	}
}
