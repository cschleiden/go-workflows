package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/google/uuid"
)

type workflowStatus int

const (
	workflowStatusRunning workflowStatus = iota
	workflowStatusCompleted
)

type workflowState struct {
	Status workflowStatus

	Instance core.WorkflowInstance

	ParentInstance *core.WorkflowInstance

	History []history.Event

	PendingEvents []history.Event

	CreatedAt   time.Time
	CompletedAt *time.Time
}

// Simple in-memory backend for development
type memoryBackend struct {
	mu               sync.Mutex
	instances        map[string]*workflowState
	lockedWorkflows  map[string]bool
	pendingWorkflows map[string]bool

	// pending workflow tasks
	workflows chan string

	// pending activity tasks
	activities chan *task.Activity
}

func NewMemoryBackend() backend.Backend {
	return &memoryBackend{
		mu:        sync.Mutex{},
		instances: make(map[string]*workflowState),

		// lockedWorkflows are currently in progress by a worker
		lockedWorkflows: make(map[string]bool),

		// Queue of pending workflow instances
		workflows:        make(chan string, 100),
		pendingWorkflows: make(map[string]bool),

		// Queue of pending activity instances
		activities: make(chan *task.Activity, 100),
	}
}

func (mb *memoryBackend) CreateWorkflowInstance(ctx context.Context, m core.WorkflowEvent) error {
	// attrs, ok := m.HistoryEvent.Attributes.(history.ExecutionStartedAttributes)
	// if !ok {
	// 	return errors.New("invalid workflow instance creation event")
	// }

	mb.mu.Lock()
	defer mb.mu.Unlock()

	if _, ok := mb.instances[m.WorkflowInstance.GetInstanceID()]; ok {
		return errors.New("workflow instance already exists")
	}

	mb.createWorkflowInstance(m)

	mb.queueWorkflowTask(m.WorkflowInstance)

	return nil
}

func (mb *memoryBackend) SignalWorkflow(ctx context.Context, wfi core.WorkflowInstance, event history.Event) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	instance, ok := mb.instances[wfi.GetInstanceID()]
	if !ok {
		return errors.New("workflow instance does not exist")
	}

	instance.PendingEvents = append(instance.PendingEvents, event)

	mb.queueWorkflowTask(wfi)

	return nil
}

func (mb *memoryBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	select {
	case <-ctx.Done():
		return nil, nil

	case id := <-mb.workflows:
		mb.mu.Lock()
		defer mb.mu.Unlock()

		delete(mb.pendingWorkflows, id)

		// Only return visible events to worker
		instance := mb.instances[id]
		newEvents := make([]history.Event, 0)
		for _, event := range instance.PendingEvents {
			if event.VisibleAt == nil || event.VisibleAt.Before(time.Now().UTC()) {
				newEvents = append(newEvents, event)
			}
		}

		if len(newEvents) == 0 {
			return nil, nil
		}

		mb.lockedWorkflows[id] = true

		// Add task to queue
		t := &task.Workflow{
			WorkflowInstance: instance.Instance,
			History:          instance.History,
			NewEvents:        newEvents,
		}

		return t, nil
	}
}

func (mb *memoryBackend) CompleteWorkflowTask(
	ctx context.Context,
	t task.Workflow,
	events []history.Event,
	workflowEvents []core.WorkflowEvent,
) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	wfi := t.WorkflowInstance

	if _, ok := mb.lockedWorkflows[wfi.GetInstanceID()]; !ok {
		return errors.New("could not find locked workflow instance")
	}

	instance := mb.instances[wfi.GetInstanceID()]

	// Remove handled events from instance
	handled := make(map[string]bool)
	for _, event := range t.NewEvents {
		handled[event.ID] = true
	}

	i := 0
	for _, event := range instance.PendingEvents {
		if !handled[event.ID] {
			instance.PendingEvents[i] = event
			i++
		}
	}
	instance.PendingEvents = instance.PendingEvents[:i]

	// Add handled events to history
	instance.History = append(instance.History, t.NewEvents...)

	// Add all events from the last execution to history
	instance.History = append(instance.History, events...)

	// Queue activities
	workflowCompleted := false

	// Check events generated in last execution for any action that needs to be taken
	for _, event := range events {
		switch event.EventType {
		case history.EventType_ActivityScheduled:
			mb.activities <- &task.Activity{
				WorkflowInstance: wfi,
				ID:               uuid.NewString(),
				Event:            event,
			}

		case history.EventType_WorkflowExecutionFinished:
			workflowCompleted = true
		}
	}

	// Handle events to be handled in the future
	for _, we := range workflowEvents {
		workflowInstance := we.WorkflowInstance
		event := we.HistoryEvent

		switch event.EventType {
		case history.EventType_TimerFired:
			// Specific to this backend implementation, schedule a timer to put a new task in the queue when the timer fires
			a := event.Attributes.(*history.TimerFiredAttributes)
			go func() {
				// Ensure a workflow task is queued when this timer should fire
				delay := a.At.Sub(time.Now().UTC())
				<-time.After(delay)

				mb.mu.Lock()
				defer mb.mu.Unlock()

				mb.queueWorkflowTask(workflowInstance)
			}()
		}

		if state, ok := mb.instances[workflowInstance.GetInstanceID()]; ok {
			state.PendingEvents = append(state.PendingEvents, event)
		} else {
			mb.createWorkflowInstance(we)
		}

		// Wake up workflow orchestration, this is idempotent so even if we have multiple events for the same instance
		// only one task will be scheduled
		mb.queueWorkflowTask(workflowInstance)
	}

	// Unlock instance
	delete(mb.lockedWorkflows, wfi.GetInstanceID())

	if !workflowCompleted {
		// New events have been added while this was being worked on
		hasNewEvents := len(instance.PendingEvents) > 0
		if hasNewEvents {
			mb.queueWorkflowTask(wfi)
		}
	} else {
		instance.Status = workflowStatusCompleted
	}

	return nil
}

func (mb *memoryBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	select {
	case <-ctx.Done():
		return nil, nil

	case t := <-mb.activities:
		return t, nil
	}
}

func (mb *memoryBackend) CompleteActivityTask(_ context.Context, wfi core.WorkflowInstance, taskID string, event history.Event) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	instance, ok := mb.instances[wfi.GetInstanceID()]
	if !ok {
		return errors.New("workflow instance does not exist")
	}

	if instance.Status == workflowStatusCompleted {
		// Ignore activity result for completed workflow
	} else {
		instance.PendingEvents = append(instance.PendingEvents, event)

		// Workflow is not currently locked, mark as ready to be picked up
		if _, ok := mb.lockedWorkflows[wfi.GetInstanceID()]; !ok {
			mb.queueWorkflowTask(wfi)
		}
	}

	return nil
}

func (mb *memoryBackend) createWorkflowInstance(m core.WorkflowEvent) *workflowState {
	state := workflowState{
		Instance:      m.WorkflowInstance,
		History:       []history.Event{},
		PendingEvents: []history.Event{m.HistoryEvent},
		CreatedAt:     time.Now().UTC(),
	}

	mb.instances[m.WorkflowInstance.GetInstanceID()] = &state

	return &state
}

func (mb *memoryBackend) queueWorkflowTask(wfi core.WorkflowInstance) {
	if _, ok := mb.instances[wfi.GetInstanceID()]; !ok {
		panic("could not find workflow instance")
	}

	// Before enqueuing task, prevent multiple tasks for the same workflow
	if mb.pendingWorkflows[wfi.GetInstanceID()] {
		return
	}

	mb.workflows <- wfi.GetInstanceID()
	mb.pendingWorkflows[wfi.GetInstanceID()] = true
}
