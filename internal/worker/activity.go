package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/ticctech/go-workflows/backend"
	"github.com/ticctech/go-workflows/internal/activity"
	"github.com/ticctech/go-workflows/internal/history"
	"github.com/ticctech/go-workflows/internal/task"
	"github.com/ticctech/go-workflows/internal/workflow"
)

type ActivityWorker interface {
	Start(context.Context) error
	WaitForCompletion() error
}

type activityWorker struct {
	backend backend.Backend

	options *Options

	activityTaskQueue    chan *task.Activity
	activityTaskExecutor activity.Executor

	wg *sync.WaitGroup

	clock clock.Clock
}

func NewActivityWorker(backend backend.Backend, registry *workflow.Registry, clock clock.Clock, options *Options) ActivityWorker {
	return &activityWorker{
		backend: backend,

		options: options,

		activityTaskQueue:    make(chan *task.Activity),
		activityTaskExecutor: activity.NewExecutor(backend.Logger(), backend.Tracer(), registry),

		wg: &sync.WaitGroup{},

		clock: clock,
	}
}

func (aw *activityWorker) Start(ctx context.Context) error {
	for i := 0; i <= aw.options.ActivityPollers; i++ {
		go aw.runPoll(ctx)
	}

	go aw.runDispatcher(ctx)

	return nil
}

func (aw *activityWorker) WaitForCompletion() error {
	aw.wg.Wait()

	return nil
}

func (aw *activityWorker) runPoll(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			task, err := aw.poll(ctx, 30*time.Second)
			if err != nil {
				log.Println("error while polling for activity task:", err)
				continue
			}

			if task != nil {
				aw.activityTaskQueue <- task
			}
		}
	}
}

func (aw *activityWorker) runDispatcher(ctx context.Context) {
	var sem chan struct{}
	if aw.options.MaxParallelActivityTasks > 0 {
		sem = make(chan struct{}, aw.options.MaxParallelActivityTasks)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case task := <-aw.activityTaskQueue:
			if sem != nil {
				sem <- struct{}{}
			}

			aw.wg.Add(1)
			go func() {
				defer aw.wg.Done()

				// Create new context to allow activities to complete when root context is canceled
				taskCtx := context.Background()
				aw.handleTask(taskCtx, task)

				if sem != nil {
					<-sem
				}
			}()
		}
	}
}

func (aw *activityWorker) handleTask(ctx context.Context, task *task.Activity) {
	// Start heartbeat while activity is running
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	go func(ctx context.Context) {
		t := time.NewTicker(aw.options.ActivityHeartbeatInterval)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := aw.backend.ExtendActivityTask(ctx, task.ID); err != nil {
					aw.backend.Logger().Panic("extending activity task", "error", err)
				}
			}
		}
	}(heartbeatCtx)

	result, err := aw.activityTaskExecutor.ExecuteActivity(ctx, task)

	cancelHeartbeat()

	var event history.Event

	if err != nil {
		event = history.NewPendingEvent(
			aw.clock.Now(),
			history.EventType_ActivityFailed,
			&history.ActivityFailedAttributes{
				Reason: err.Error(),
			},
			history.ScheduleEventID(task.Event.ScheduleEventID),
		)
	} else {
		event = history.NewPendingEvent(
			aw.clock.Now(),
			history.EventType_ActivityCompleted,
			&history.ActivityCompletedAttributes{
				Result: result,
			},
			history.ScheduleEventID(task.Event.ScheduleEventID))
	}

	if err := aw.backend.CompleteActivityTask(ctx, task.WorkflowInstance, task.ID, event); err != nil {
		aw.backend.Logger().Panic("completing activity task", "error", err)
	}
}

func (aw *activityWorker) poll(ctx context.Context, timeout time.Duration) (*task.Activity, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var task *task.Activity
	var err error

	done := make(chan struct{})

	go func() {
		task, err = aw.backend.GetActivityTask(ctx)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil, nil
	case <-done:
		return task, err
	}
}
