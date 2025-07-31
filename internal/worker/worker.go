package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
)

//go:generate mockery --name=TaskWorker --inpackage
type TaskWorker[Task, Result any] interface {
	Start(context.Context, []workflow.Queue) error
	Get(context.Context, []workflow.Queue) (*Task, error)
	Extend(context.Context, *Task) error
	Execute(context.Context, *Task) (*Result, error)
	Complete(context.Context, *Result, *Task) error
}

type Worker[Task, TaskResult any] struct {
	options *WorkerOptions

	tw TaskWorker[Task, TaskResult]

	taskQueue chan timestampedTask[Task]

	logger *slog.Logger

	pollersWg sync.WaitGroup

	dispatcherDone chan struct{}

	currentTime func() time.Time // default is time.Now, can be overwritten for testing
}

type WorkerOptions struct {
	Pollers int

	MaxParallelTasks int

	HeartbeatInterval time.Duration

	PollingInterval time.Duration

	Queues []workflow.Queue
}

type timestampedTask[Task any] struct {
	task     *Task
	pollTime time.Time
}

func NewWorker[Task, TaskResult any](
	b backend.Backend, tw TaskWorker[Task, TaskResult], options *WorkerOptions,
) *Worker[Task, TaskResult] {
	// If no queues given, add the default queue
	if len(options.Queues) == 0 {
		options.Queues = append(options.Queues, workflow.QueueDefault)
	}

	// Always include system queue
	if !slices.Contains(options.Queues, core.QueueSystem) {
		options.Queues = append(options.Queues, core.QueueSystem)
	}

	return &Worker[Task, TaskResult]{
		tw:             tw,
		options:        options,
		taskQueue:      make(chan timestampedTask[Task]),
		logger:         b.Options().Logger,
		dispatcherDone: make(chan struct{}, 1),
		currentTime:    time.Now,
	}
}

func (w *Worker[Task, TaskResult]) Start(ctx context.Context) error {
	if err := w.tw.Start(ctx, w.options.Queues); err != nil {
		return fmt.Errorf("starting task worker: %w", err)
	}

	w.pollersWg.Add(w.options.Pollers)

	for i := 0; i < w.options.Pollers; i++ {
		go w.poller(ctx)
	}

	go w.dispatcher()

	return nil
}

func (w *Worker[Task, TaskResult]) WaitForCompletion() error {
	// Wait for task pollers to finish
	w.pollersWg.Wait()

	// Wait for tasks to finish
	close(w.taskQueue)
	<-w.dispatcherDone

	return nil
}

func (w *Worker[Task, TaskResult]) poller(ctx context.Context) {
	defer w.pollersWg.Done()

	var ticker *time.Ticker

	if w.options.PollingInterval > 0 {
		ticker = time.NewTicker(w.options.PollingInterval)
		defer ticker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		task, err := w.poll(ctx, 30*time.Second)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				w.logger.ErrorContext(ctx, "error polling task", "error", err)
			}
		} else if task != nil {
			w.taskQueue <- timestampedTask[Task]{task: task, pollTime: w.currentTime()}
			continue // check for new tasks right away
		}

		if w.options.PollingInterval > 0 {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (w *Worker[Task, TaskResult]) dispatcher() {
	var sem chan struct{}

	if w.options.MaxParallelTasks > 0 {
		sem = make(chan struct{}, w.options.MaxParallelTasks)
	}

	var wg sync.WaitGroup

	for tt := range w.taskQueue {
		// If limited max tasks, wait for a slot to open up
		if sem != nil {
			sem <- struct{}{}
		}

		wg.Add(1)

		tt := tt
		go func() {
			defer wg.Done()

			// Create new context to allow tasks to complete when root context is canceled
			taskCtx := context.Background()
			if err := w.handle(taskCtx, tt); err != nil {
				w.logger.ErrorContext(taskCtx, "error handling task", "error", err)
			}

			if sem != nil {
				<-sem
			}
		}()
	}

	wg.Wait()

	w.dispatcherDone <- struct{}{}
}

func (w *Worker[Task, TaskResult]) handle(ctx context.Context, tt timestampedTask[Task]) error {
	ctx, cancelTask := context.WithCancel(ctx)
	defer cancelTask()
	if w.options.HeartbeatInterval > 0 {
		// Start heartbeat while processing task
		go func() {
			// If heartbeat stops, cancel the task.
			// A stopped heartbeat indicates that the backend
			// no longer recognizes our ownership of the task.
			defer cancelTask()
			w.heartbeatTask(ctx, tt)
		}()
	}

	result, err := w.tw.Execute(ctx, tt.task)
	if err != nil {
		return fmt.Errorf("executing task: %w", err)
	}

	return w.tw.Complete(ctx, result, tt.task)
}

func (w *Worker[Task, TaskResult]) heartbeatTask(ctx context.Context, tt timestampedTask[Task]) {
	// For first heartbeat, check how long it's been since
	// the task was polled.
	timeSincePoll := w.currentTime().Sub(tt.pollTime)
	select {
	case <-ctx.Done():
		return
	case <-time.After(w.options.HeartbeatInterval - timeSincePoll):
		if !w.extend(ctx, tt.task) {
			return
		}
	}

	// Afterwards, heartbeat at regular intervals
	t := time.NewTicker(w.options.HeartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if !w.extend(ctx, tt.task) {
				return
			}
		}
	}
}

// extend returns whether or not the task was extended.
func (w *Worker[Task, TaskResult]) extend(ctx context.Context, t *Task) bool {
	if err := w.tw.Extend(ctx, t); err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() == context.Canceled {
			// If upstream ctx cancellation, this is because the
			// task completed and we are simply cleaning up
			// the heartbeat goroutine. Nothing interesting to log.
			return false
		}
		w.logger.ErrorContext(ctx, "could not heartbeat task", "error", err)
		return false
	}
	return true
}

func (w *Worker[Task, TaskResult]) poll(ctx context.Context, timeout time.Duration) (*Task, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	task, err := w.tw.Get(ctx, w.options.Queues)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, nil
		}

		return nil, err
	}

	return task, nil
}
