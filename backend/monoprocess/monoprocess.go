package monoprocess

import (
	"context"
	"log/slog"
	"reflect"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/workflow"
)

type monoprocessBackend struct {
	backend.Backend

	workflowSignal chan struct{}
	activitySignal chan struct{}

	logger *slog.Logger
}

var _ backend.Backend = (*monoprocessBackend)(nil)

// NewMonoprocessBackend wraps an existing backend and improves its responsiveness
// in case the backend and worker are running in the same process. This backend
// uses channels to notify the worker every time there is a new task ready to be
// worked on. Note that only one worker will be notified.
// IMPORTANT: Only use this backend when the backend and worker are running in
// the same process.
func NewMonoprocessBackend(b backend.Backend) *monoprocessBackend {
	mb := &monoprocessBackend{
		Backend:        b,
		workflowSignal: make(chan struct{}, 1),
		activitySignal: make(chan struct{}, 1),
		logger:         b.Options().Logger,
	}
	return mb
}

func (b *monoprocessBackend) Options() *backend.Options {
	return b.Backend.Options()
}

func (b *monoprocessBackend) GetWorkflowTask(ctx context.Context, queues []workflow.Queue) (*backend.WorkflowTask, error) {
	// loop until either we find a task or the context is cancelled
	for {
		if w, err := b.Backend.GetWorkflowTask(ctx, queues); w != nil || err != nil {
			return w, err
		}
		b.logger.DebugContext(ctx, "worker waiting for workflow task signal")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-b.workflowSignal:
			b.logger.DebugContext(ctx, "worker got a workflow task signal")
		}
	}
}

func (b *monoprocessBackend) GetActivityTask(ctx context.Context, queues []workflow.Queue) (*backend.ActivityTask, error) {
	// loop until either we find a task or the context is cancelled
	for {
		if a, err := b.Backend.GetActivityTask(ctx, queues); a != nil || err != nil {
			return a, err
		}
		b.logger.DebugContext(ctx, "worker waiting for activity task signal")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-b.activitySignal:
			b.logger.DebugContext(ctx, "worker got an activity task signal")
		}
	}
}

func (b *monoprocessBackend) CreateWorkflowInstance(ctx context.Context, queue workflow.Queue, instance *workflow.Instance, event *history.Event) error {
	if err := b.Backend.CreateWorkflowInstance(ctx, queue, instance, event); err != nil {
		return err
	}
	b.notifyWorkflowWorker(ctx)
	return nil
}

func (b *monoprocessBackend) CompleteWorkflowTask(
	ctx context.Context,
	task *backend.WorkflowTask,
	state core.WorkflowInstanceState,
	executedEvents, activityEvents, timerEvents []*history.Event,
	workflowEvents []*history.WorkflowEvent,
) error {
	if err := b.Backend.CompleteWorkflowTask(ctx, task, state, executedEvents, activityEvents, timerEvents, workflowEvents); err != nil {
		return err
	}

	if len(activityEvents) > 0 {
		b.notifyActivityWorker(ctx)
	}

	for _, e := range timerEvents {
		attr, ok := e.Attributes.(*history.TimerFiredAttributes)
		if !ok {
			b.logger.WarnContext(ctx, "unknown attributes type in timer event", "type", reflect.TypeOf(e.Attributes).String())
			continue
		}
		b.logger.DebugContext(ctx, "scheduling timer to notify workflow worker")
		// Note that the worker will be notified even if the timer event gets
		// cancelled. This is ok, because the poller will simply find no task
		// and continue.
		time.AfterFunc(time.Until(attr.At), func() {
			b.notifyWorkflowWorker(ctx)
		})
	}

	b.notifyWorkflowWorker(ctx)
	return nil
}

func (b *monoprocessBackend) CompleteActivityTask(ctx context.Context, task *backend.ActivityTask, result *history.Event) error {
	if err := b.Backend.CompleteActivityTask(ctx, task, result); err != nil {
		return err
	}
	b.notifyWorkflowWorker(ctx)
	return nil
}

func (b *monoprocessBackend) CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance, cancelEvent *history.Event) error {
	if err := b.Backend.CancelWorkflowInstance(ctx, instance, cancelEvent); err != nil {
		return err
	}
	b.notifyWorkflowWorker(ctx)
	return nil
}

func (b *monoprocessBackend) SignalWorkflow(ctx context.Context, instanceID string, event *history.Event) error {
	if err := b.Backend.SignalWorkflow(ctx, instanceID, event); err != nil {
		return err
	}
	b.notifyWorkflowWorker(ctx)
	return nil
}

func (b *monoprocessBackend) notifyActivityWorker(ctx context.Context) {
	select {
	case b.activitySignal <- struct{}{}:
		b.logger.DebugContext(ctx, "signalled a new activity task to worker")
	default:
		// the signal channel already contains a signal, no need to add another
	}
}

func (b *monoprocessBackend) notifyWorkflowWorker(ctx context.Context) {
	select {
	case b.workflowSignal <- struct{}{}:
		b.logger.DebugContext(ctx, "signalled a new workflow task to worker")
	default:
		// the signal channel already contains a signal, no need to add another
	}
}
