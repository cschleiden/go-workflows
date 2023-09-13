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

func NewMonoprocessBackend(b backend.Backend, signalBufferSize int, signalTimeout time.Duration) *monoprocessBackend {
	if signalTimeout <= 0 {
		signalTimeout = time.Second
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
	b.logger.Debug("worker waiting for workflow task signal")
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.workflowSignal:
		b.logger.Debug("worker got a workflow task signal")
		return b.GetWorkflowTask(ctx)
	}
}

func (b *monoprocessBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	if a, err := b.Backend.GetActivityTask(ctx); a != nil || err != nil {
		return a, err
	}
	b.logger.Debug("worker waiting for activity task signal")
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.activitySignal:
		b.logger.Debug("worker got an activity task signal")
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
			break // no reason to notify more
		}
	}
	for _, e := range timerEvents {
		attr, ok := e.Attributes.(*history.TimerFiredAttributes)
		if !ok {
			b.logger.Warn("unknown attributes type in timer event", "type", reflect.TypeOf(e.Attributes).String())
			continue
		}
		b.logger.Debug("scheduling timer to notify workflow worker")
		time.AfterFunc(attr.At.Sub(time.Now()), func() { b.notifyWorkflowWorker(ctx) }) // TODO: cancel timer if the event gets cancelled
	}
	for _, e := range workflowEvents {
		if e.HistoryEvent.Type != history.EventType_WorkflowExecutionStarted &&
			e.HistoryEvent.Type != history.EventType_SubWorkflowCompleted &&
			e.HistoryEvent.Type != history.EventType_WorkflowExecutionCanceled {
			continue
		}
		if !b.notifyWorkflowWorker(ctx) {
			break // no reason to notify more
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
	select {
	case <-ctx.Done():
		// we didn't manage to notify the worker that there is a new task, it
		// will pick it up after the poll timeout
		b.logger.Debug("failed to signal activity task to worker, context cancelled")
	case <-time.After(b.signalTimeout):
		// we didn't manage to notify the worker that there is a new task, it
		// will pick it up after the poll timeout
		b.logger.Debug("failed to signal activity task to worker, timeout")
	case b.activitySignal <- struct{}{}:
		b.logger.Debug("signalled a new activity task to worker")
		return true
	}
	return false
}

func (b *monoprocessBackend) notifyWorkflowWorker(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		// we didn't manage to notify the worker that there is a new task, it
		// will pick it up after the poll timeout
		b.logger.Debug("failed to signal workflow task to worker, context cancelled")
	case <-time.After(b.signalTimeout):
		// we didn't manage to notify the worker that there is a new task, it
		// will pick it up after the poll timeout
		b.logger.Debug("failed to signal workflow task to worker, timeout")
	case b.workflowSignal <- struct{}{}:
		b.logger.Debug("signalled a new workflow task to worker")
		return true
	}
	return false
}
