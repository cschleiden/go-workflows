package worker

import (
	"context"
	"log"
	"time"

	"github.com/cschleiden/go-dt/internal/activity"
	"github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
)

type ActivityWorker interface {
	Start(context.Context) error
}

type activityWorker struct {
	backend backend.Backend

	activityTaskQueue    chan task.Activity
	activityTaskExecutor activity.Executor

	logger *log.Logger
}

func NewActivityWorker(backend backend.Backend, registry *workflow.Registry) ActivityWorker {
	return &activityWorker{
		backend: backend,

		activityTaskQueue:    make(chan task.Activity),
		activityTaskExecutor: activity.NewExecutor(registry),

		logger: log.Default(),
	}
}

func (ww *activityWorker) Start(ctx context.Context) error {
	go ww.runPoll(ctx)

	go ww.runDispatcher(ctx)

	return nil
}

func (ww *activityWorker) runPoll(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			task, err := ww.poll(ctx, 30*time.Second)
			if err != nil {
				log.Println("error while polling for activity task:", err)
			} else if task != nil {
				ww.activityTaskQueue <- *task
			}
		}
	}
}

func (ww *activityWorker) runDispatcher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-ww.activityTaskQueue:
			go ww.handleTask(ctx, task)
		}
	}
}

func (ww *activityWorker) handleTask(ctx context.Context, task task.Activity) {
	result, err := ww.activityTaskExecutor.ExecuteActivity(ctx, task)

	var event history.Event

	if err != nil {
		event = history.NewHistoryEvent(
			history.EventType_ActivityFailed,
			task.Event.EventID,
			&history.ActivityFailedAttributes{
				Reason: err.Error(),
			})
	} else {
		event = history.NewHistoryEvent(
			history.EventType_ActivityCompleted,
			task.Event.EventID,
			&history.ActivityCompletedAttributes{
				Result: result,
			})
	}

	if err := ww.backend.CompleteActivityTask(ctx, task.WorkflowInstance, task.ID, event); err != nil {
		panic(err)
	}
}

func (ww *activityWorker) poll(ctx context.Context, timeout time.Duration) (*task.Activity, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var task *task.Activity
	var err error

	done := make(chan struct{})

	go func() {
		task, err = ww.backend.GetActivityTask(ctx)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil, nil
	case <-done:
		return task, err
	}
}
