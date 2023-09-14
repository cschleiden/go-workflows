package monoprocess

import (
	"context"
	"log/slog"
	"reflect"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/workflow"
)

type monoprocessBackend struct {
	backend.Backend

	workflowSignal chan struct{}
	activitySignal chan struct{}
	signalTimeout  time.Duration

	logger *slog.Logger
}

// NewMonoprocessBackend wraps an existing backend and improves its responsiveness
// in case the backend and worker are running in the same process. This backend
// uses channels to notify the worker every time there is a new task ready to be
// worked on. Note that only one worker will be notified.
// IMPORTANT: Only use this backend if the backend and worker are running in the
// same process.
func NewMonoprocessBackend(b backend.Backend, signalBufferSize int, signalTimeout time.Duration) *monoprocessBackend {
	if signalTimeout <= 0 {
		signalTimeout = time.Second // default
	}
	mb := &monoprocessBackend{
		Backend:        b,
		workflowSignal: make(chan struct{}, signalBufferSize),
		activitySignal: make(chan struct{}, signalBufferSize),
		signalTimeout:  signalTimeout,
		logger:         b.Logger(),
	}
	return mb
}

func (b *monoprocessBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	if w, err := b.Backend.GetWorkflowTask(ctx); w != nil || err != nil {
		return w, err
	}
	b.logger.DebugContext(ctx, "worker waiting for workflow task signal")
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.workflowSignal:
		b.logger.DebugContext(ctx, "worker got a workflow task signal")
		return b.GetWorkflowTask(ctx)
	}
}

func (b *monoprocessBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	if a, err := b.Backend.GetActivityTask(ctx); a != nil || err != nil {
		return a, err
	}
	b.logger.DebugContext(ctx, "worker waiting for activity task signal")
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.activitySignal:
		b.logger.DebugContext(ctx, "worker got an activity task signal")
		return b.GetActivityTask(ctx)
	}
}

func (b *monoprocessBackend) CreateWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error {
	if err := b.Backend.CreateWorkflowInstance(ctx, instance, event); err != nil {
		return err
	}
	b.notifyWorkflowWorker(ctx)
	return nil
}

func (b *monoprocessBackend) CompleteWorkflowTask(
	ctx context.Context,
	task *task.Workflow,
	instance *workflow.Instance,
	state core.WorkflowInstanceState,
	executedEvents, activityEvents, timerEvents []*history.Event,
	workflowEvents []history.WorkflowEvent,
) error {
	if err := b.Backend.CompleteWorkflowTask(ctx, task, instance, state, executedEvents, activityEvents, timerEvents, workflowEvents); err != nil {
		return err
	}
	for _, e := range activityEvents {
		if e.Type != history.EventType_ActivityScheduled {
			continue
		}
		if !b.notifyActivityWorker(ctx) {
			break // no reason to notify more, queue is full
		}
	}
	for _, e := range timerEvents {
		attr, ok := e.Attributes.(*history.TimerFiredAttributes)
		if !ok {
			b.logger.Warn("unknown attributes type in timer event", "type", reflect.TypeOf(e.Attributes).String())
			continue
		}
		b.logger.DebugContext(ctx, "scheduling timer to notify workflow worker")
		time.AfterFunc(attr.At.Sub(time.Now()), func() { b.notifyWorkflowWorker(ctx) }) // TODO: cancel timer if the event gets cancelled
	}
	for _, e := range workflowEvents {
		if e.HistoryEvent.Type != history.EventType_WorkflowExecutionStarted &&
			e.HistoryEvent.Type != history.EventType_SubWorkflowCompleted &&
			e.HistoryEvent.Type != history.EventType_WorkflowExecutionCanceled {
			continue
		}
		if !b.notifyWorkflowWorker(ctx) {
			break // no reason to notify more, queue is full
		}
	}
	return nil
}

func (b *monoprocessBackend) CompleteActivityTask(ctx context.Context, instance *workflow.Instance, activityID string, event *history.Event) error {
	if err := b.Backend.CompleteActivityTask(ctx, instance, activityID, event); err != nil {
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

func (b *monoprocessBackend) notifyActivityWorker(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, b.signalTimeout)
	defer cancel()
	select {
	case <-ctx.Done():
		// we didn't manage to notify the worker that there is a new task, it
		// will pick it up after the poll timeout
		b.logger.DebugContext(ctx, "failed to signal activity task to worker", "reason", ctx.Err())
		return false
	case b.activitySignal <- struct{}{}:
		b.logger.DebugContext(ctx, "signalled a new activity task to worker")
		return true
	}
}

func (b *monoprocessBackend) notifyWorkflowWorker(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, b.signalTimeout)
	defer cancel()
	select {
	case <-ctx.Done():
		// we didn't manage to notify the worker that there is a new task, it
		// will pick it up after the poll timeout
		b.logger.DebugContext(ctx, "failed to signal workflow task to worker", "reason", ctx.Err())
		return false
	case b.workflowSignal <- struct{}{}:
		b.logger.DebugContext(ctx, "signalled a new workflow task to worker")
		return true
	}
}
