package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cschleiden/go-workflows/backend"
)

type TaskWorker[Task, Result any] interface {
	Get(context.Context) (*Task, error)
	Extend(context.Context, *Task) error
	Execute(context.Context, *Task) (*Result, error)
	Complete(context.Context, *Result, *Task) error
}

type Worker[Task, TaskResult any] struct {
	options *WorkerOptions

	tw TaskWorker[Task, TaskResult]

	taskQueue chan *Task

	logger *slog.Logger

	pollersWg sync.WaitGroup

	dispatcherDone chan struct{}
}

type WorkerOptions struct {
	Pollers int

	MaxParallelTasks int

	HeartbeatInterval time.Duration

	PollingInterval time.Duration
}

func NewWorker[Task, TaskResult any](
	b backend.Backend, tw TaskWorker[Task, TaskResult], options *WorkerOptions,
) *Worker[Task, TaskResult] {
	return &Worker[Task, TaskResult]{
		tw:             tw,
		options:        options,
		taskQueue:      make(chan *Task),
		logger:         b.Logger(),
		dispatcherDone: make(chan struct{}, 1),
	}
}

func (w *Worker[Task, TaskResult]) Start(ctx context.Context) error {
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

	ticker := time.NewTicker(w.options.PollingInterval)
	defer ticker.Stop()

	for {
		task, err := w.poll(ctx, 30*time.Second)
		if err != nil {
			w.logger.ErrorContext(ctx, "error polling task", "error", err)
		} else if task != nil {
			w.taskQueue <- task
			continue // check for new tasks right away
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker[Task, TaskResult]) dispatcher() {
	var sem chan struct{}

	if w.options.MaxParallelTasks > 0 {
		sem = make(chan struct{}, w.options.MaxParallelTasks)
	}

	var wg sync.WaitGroup

	for t := range w.taskQueue {
		// If limited max tasks, wait for a slot to open up
		if sem != nil {
			sem <- struct{}{}
		}

		wg.Add(1)

		t := t
		go func() {
			defer wg.Done()

			// Create new context to allow tasks to complete when root context is canceled
			taskCtx := context.Background()
			w.handle(taskCtx, t)

			if sem != nil {
				<-sem
			}
		}()
	}

	wg.Wait()

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

	task, err := w.tw.Get(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, nil
		}

		return nil, err
	}

	return task, nil
}
