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

// TODO: This will have to move somewhere else
type workflowState struct {
	Instance core.WorkflowInstance

	History []history.HistoryEvent

	NewEvents []history.HistoryEvent

	Created time.Time
}

// Simple in-memory backend for development
type memoryBackend struct {
	mu              sync.Mutex
	instances       map[string]*workflowState
	lockedWorkflows map[string]bool

	// pending workflow tasks not yet picked up
	workflows chan *task.Workflow

	// pending activity tasks
	activities chan *task.Activity
}

func NewMemoryBackend() backend.Backend {
	return &memoryBackend{
		mu:              sync.Mutex{},
		instances:       make(map[string]*workflowState),
		lockedWorkflows: make(map[string]bool),

		// Queue of pending workflow instances
		workflows:  make(chan *task.Workflow, 100),
		activities: make(chan *task.Activity, 100),
	}
}

func (mb *memoryBackend) CreateWorkflowInstance(ctx context.Context, m core.TaskMessage) error {
	// attrs, ok := m.HistoryEvent.Attributes.(history.ExecutionStartedAttributes)
	// if !ok {
	// 	return errors.New("invalid workflow instance creation event")
	// }

	mb.mu.Lock()
	defer mb.mu.Unlock()

	if _, ok := mb.instances[m.WorkflowInstance.GetInstanceID()]; ok {
		return errors.New("workflow instance already exists")
	}

	state := workflowState{
		Instance:  m.WorkflowInstance,
		History:   []history.HistoryEvent{m.HistoryEvent},
		NewEvents: []history.HistoryEvent{},
		Created:   time.Now().UTC(),
	}

	mb.instances[m.WorkflowInstance.GetInstanceID()] = &state

	// Add to queue
	mb.workflows <- &task.Workflow{
		WorkflowInstance: state.Instance,
		History:          state.History,
	}

	return nil
}

func (mb *memoryBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	select {
	case <-ctx.Done():
		return nil, nil

	case t := <-mb.workflows:
		mb.mu.Lock()
		defer mb.mu.Unlock()

		mb.lockedWorkflows[t.WorkflowInstance.GetInstanceID()] = true

		return t, nil
	}
}

func (mb *memoryBackend) CompleteWorkflowTask(_ context.Context, t task.Workflow, newEvents []history.HistoryEvent) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	wfi := t.WorkflowInstance

	if _, ok := mb.lockedWorkflows[wfi.GetInstanceID()]; !ok {
		return errors.New("could not find locked workflow instance")
	}

	// Unlock instance
	delete(mb.lockedWorkflows, wfi.GetInstanceID())

	instance := mb.instances[wfi.GetInstanceID()]

	// Remove handled events from instance
	handled := make(map[int]bool)
	for _, event := range t.NewEvents {
		handled[event.EventID] = true
	}

	i := 0
	for _, event := range instance.NewEvents {
		if !handled[event.EventID] {
			instance.NewEvents[i] = event
			i++
		} else {
			// Event handled, add to history
			instance.History = append(instance.History, event)
			event.Played = true // TOOD: Have the caller determine this?
		}
	}

	// Queue activities
	for _, event := range newEvents {
		switch event.EventType {
		case history.HistoryEventType_ActivityScheduled:
			mb.activities <- &task.Activity{
				WorkflowInstance: wfi,
				ID:               uuid.NewString(),
				Event:            event,
			}
		}

		instance.History = append(instance.History, event)
	}

	// New events from this checkpoint or added while this was locked
	hasNewEvents := len(instance.NewEvents) > 0
	if hasNewEvents {
		mb.addWorkflowTask(wfi)
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

func (mb *memoryBackend) CompleteActivityTask(_ context.Context, wfi core.WorkflowInstance, taskID string, event history.HistoryEvent) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	instance, ok := mb.instances[wfi.GetInstanceID()]
	if !ok {
		panic("could not find workflow instance")
	}

	instance.NewEvents = append(instance.NewEvents, event)

	// Workflow is not currently locked, mark as ready to be picked up
	if _, ok := mb.lockedWorkflows[wfi.GetInstanceID()]; !ok {
		mb.addWorkflowTask(wfi)
	}

	return nil
}

func (mb *memoryBackend) addWorkflowTask(wfi core.WorkflowInstance) {
	instance, ok := mb.instances[wfi.GetInstanceID()]
	if !ok {
		panic("could not find workflow instance")
	}

	// TODO: Only include messages which should be visible right now

	// Add task to queue
	mb.workflows <- &task.Workflow{
		WorkflowInstance: wfi,
		History:          instance.History,
		NewEvents:        instance.NewEvents,
	}
}
