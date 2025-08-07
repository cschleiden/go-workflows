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

	taskQueue *workQueue[Task]

	logger *slog.Logger

	pollersWg sync.WaitGroup

	dispatcherDone chan struct{}
}

type WorkerOptions struct {
	Pollers int

	MaxParallelTasks int

	HeartbeatInterval time.Duration

	PollingInterval time.Duration

	Queues []workflow.Queue
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
		taskQueue:      newWorkQueue[Task](options.MaxParallelTasks),
		logger:         b.Options().Logger,
		dispatcherDone: make(chan struct{}, 1),
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
	close(w.taskQueue.tasks)
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

		// Reserve slot for work we might get. This blocks if there are no slots available.
		if err := w.taskQueue.reserve(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
		}

		task, err := w.poll(ctx, 30*time.Second)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				w.logger.ErrorContext(ctx, "error polling task", "error", err)
			}
		} else if task != nil {
			if err := w.taskQueue.add(ctx, task); err != nil {
				if !errors.Is(err, context.Canceled) {
					w.logger.ErrorContext(ctx, "error adding task to queue", "error", err)
					w.taskQueue.release()
				}
			}
			continue // check for new tasks right away
		} else {
			// Did not use the reserved slot, release
			w.taskQueue.release()
		}

		// Optionally wait between unsuccessful polling attempts
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
	var wg sync.WaitGroup

	for t := range w.taskQueue.tasks {
		wg.Add(1)

		t := t
		go func() {
			defer w.taskQueue.release()
			defer wg.Done()

			// Create new context to allow tasks to complete when root context is canceled
			taskCtx := context.Background()
			if err := w.handle(taskCtx, t); err != nil {
				w.logger.ErrorContext(taskCtx, "error handling task", "error", err)
			}
		}()
	}

	// Wait for all pending tasks to finish
	wg.Wait()

	// Then notify anyone waiting for this that the dispatcher is done.
	w.dispatcherDone <- struct{}{}
}

func (w *Worker[Task, TaskResult]) handle(ctx context.Context, t *Task) error {
	if w.options.HeartbeatInterval > 0 {
		// Start heartbeat while processing task
		heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
		defer cancelHeartbeat()
		go w.heartbeatTask(heartbeatCtx, t)
	}

	result, err := w.tw.Execute(ctx, t)
	if err != nil {
		return fmt.Errorf("executing task: %w", err)
	}

	return w.tw.Complete(ctx, result, t)
}

func (w *Worker[Task, TaskResult]) heartbeatTask(ctx context.Context, task *Task) {
	t := time.NewTicker(w.options.HeartbeatInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := w.tw.Extend(ctx, task); err != nil {
				w.logger.ErrorContext(ctx, "could not heartbeat task", "error", err)
			}
		}
	}
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
