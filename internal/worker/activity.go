package worker

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/activity"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/metrickeys"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/cschleiden/go-workflows/metrics"
)

type ActivityWorker struct {
	backend backend.Backend

	options *Options

	activityTaskQueue    chan *task.Activity
	activityTaskExecutor activity.Executor

	wg *sync.WaitGroup

	clock clock.Clock
}

func NewActivityWorker(backend backend.Backend, registry *workflow.Registry, clock clock.Clock, options *Options) *ActivityWorker {
	return &ActivityWorker{
		backend: backend,

		options: options,

		activityTaskQueue:    make(chan *task.Activity),
		activityTaskExecutor: activity.NewExecutor(backend.Logger(), backend.Tracer(), backend.Converter(), registry),

		wg: &sync.WaitGroup{},

		clock: clock,
	}
}

func (aw *ActivityWorker) Start(ctx context.Context) error {
	for i := 0; i <= aw.options.ActivityPollers; i++ {
		go aw.runPoll(ctx)
	}

	go aw.runDispatcher(context.Background())

	return nil
}

func (aw *ActivityWorker) WaitForCompletion() error {
	close(aw.activityTaskQueue)

	aw.wg.Wait()

	return nil
}

func (aw *ActivityWorker) runPoll(ctx context.Context) {
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

func (aw *ActivityWorker) runDispatcher(ctx context.Context) {
	var sem chan struct{}
	if aw.options.MaxParallelActivityTasks > 0 {
		sem = make(chan struct{}, aw.options.MaxParallelActivityTasks)
	}

	for task := range aw.activityTaskQueue {
		if sem != nil {
			sem <- struct{}{}
		}

		task := task

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

func (aw *ActivityWorker) handleTask(ctx context.Context, task *task.Activity) {
	a := task.Event.Attributes.(*history.ActivityScheduledAttributes)
	ametrics := aw.backend.Metrics().WithTags(metrics.Tags{metrickeys.ActivityName: a.Name})

	// Record how long this task was in the queue
	scheduledAt := task.Event.Timestamp
	timeInQueue := time.Since(scheduledAt)
	ametrics.Distribution(metrickeys.ActivityTaskDelay, metrics.Tags{}, float64(timeInQueue/time.Millisecond))

	// Start heartbeat while activity is running
	if aw.options.ActivityHeartbeatInterval > 0 {
		heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
		defer cancelHeartbeat()

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
	}

	timer := metrics.Timer(ametrics, metrickeys.ActivityTaskProcessed, metrics.Tags{})
	defer timer.Stop()

	result, err := aw.activityTaskExecutor.ExecuteActivity(ctx, task)

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

func (aw *ActivityWorker) poll(ctx context.Context, timeout time.Duration) (*task.Activity, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	task, err := aw.backend.GetActivityTask(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, nil
		}
	}

	return task, nil
}
