package worker

import "context"

type workQueue[Task any] struct {
	tasks       chan *Task
	slots       chan struct{}
	emptyNotify chan struct{}
}

func newWorkQueue[Task any](maxParallelTasks int) *workQueue[Task] {
	var slots chan struct{}
	if maxParallelTasks > 0 {
		slots = make(chan struct{}, maxParallelTasks)
	}

	return &workQueue[Task]{
		tasks: make(chan *Task),
		slots: slots,
	}
}

func (w *workQueue[Task]) reserve(ctx context.Context) error {
	if w.slots == nil {
		return nil // No limit on parallel tasks, no reservation needed
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case w.slots <- struct{}{}:
		return nil
	}
}

func (w *workQueue[Task]) add(ctx context.Context, task *Task) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case w.tasks <- task:
		return nil
	}
}

func (w *workQueue[Task]) release() {
	if w.slots == nil {
		return
	}

	<-w.slots
}
