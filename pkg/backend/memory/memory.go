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

	mb.queueWorkflowTask(m.WorkflowInstance.GetInstanceID())

	return nil
}

func (mb *memoryBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	instance, ok := mb.instances[instanceID]
	if !ok {
		return errors.New("workflow instance does not exist")
	}

	instance.PendingEvents = append(instance.PendingEvents, event)

	mb.queueWorkflowTask(instanceID)

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
		newEvents := instance.getNewEvents()

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
	wfi core.WorkflowInstance,
	executedEvents []history.Event,
	workflowEvents []core.WorkflowEvent,
) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if !mb.lockedWorkflows[wfi.GetInstanceID()] {
		return errors.New("could not find locked workflow instance")
	}

	instance := mb.instances[wfi.GetInstanceID()]

	// Remove handled events from pending events
	handled := make(map[string]bool)
	for _, event := range executedEvents {
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

	// Add all events from the last execution to history
	instance.History = append(instance.History, executedEvents...)

	// Queue activities
	workflowCompleted := false

	// Check events generated in last execution for any action that needs to be taken
	for _, event := range executedEvents {
		switch event.Type {
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

		switch event.Type {
		case history.EventType_TimerFired:
			// Specific to this backend implementation, schedule a timer to put a new task in the queue when the timer fires
			a := event.Attributes.(*history.TimerFiredAttributes)
			go func() {
				// Ensure a workflow task is queued when this timer should fire
				delay := a.At.Sub(time.Now().UTC())
				<-time.After(delay)

				mb.mu.Lock()
				defer mb.mu.Unlock()

				mb.queueWorkflowTask(workflowInstance.GetInstanceID())
			}()
		}

		if state, ok := mb.instances[workflowInstance.GetInstanceID()]; ok {
			state.PendingEvents = append(state.PendingEvents, event)
		} else {
			mb.createWorkflowInstance(we)
		}

		// Wake up workflow orchestration, this is idempotent so even if we have multiple events for the same instance
		// only one task will be scheduled
		mb.queueWorkflowTask(workflowInstance.GetInstanceID())
	}

	// Unlock instance
	delete(mb.lockedWorkflows, wfi.GetInstanceID())

	if !workflowCompleted {
		// New events have been added while this was being worked on
		hasNewEvents := len(instance.getNewEvents()) > 0
		if hasNewEvents {
			mb.queueWorkflowTask(wfi.GetInstanceID())
		}
	} else {
		instance.Status = workflowStatusCompleted
	}

	return nil
}

func (mb *memoryBackend) ExtendWorkflowTask(ctx context.Context, instance core.WorkflowInstance) error {
	// No need to extend workflow task, return immediately
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

func (mb *memoryBackend) CompleteActivityTask(ctx context.Context, wfi core.WorkflowInstance, taskID string, event history.Event) error {
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
			mb.queueWorkflowTask(wfi.GetInstanceID())
		}
	}

	return nil
}

func (mb *memoryBackend) ExtendActivityTask(ctx context.Context, activityID string) error {
	// No locking here, just return
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

func (mb *memoryBackend) queueWorkflowTask(instanceID string) {
	if _, ok := mb.instances[instanceID]; !ok {
		panic("could not find workflow instance")
	}

	// Before enqueuing task, prevent multiple tasks for the same workflow
	if mb.pendingWorkflows[instanceID] {
		return
	}

	mb.workflows <- instanceID
	mb.pendingWorkflows[instanceID] = true
}
