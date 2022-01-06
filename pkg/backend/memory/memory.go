package memory

import (
	"context"
	"errors"
	"io"
	"log"
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
	status workflowStatus

	Instance core.WorkflowInstance

	History []history.HistoryEvent

	NewEvents []history.HistoryEvent

	Created time.Time
}

// Simple in-memory backend for development
type memoryBackend struct {
	mu               sync.Mutex
	instances        map[string]*workflowState
	lockedWorkflows  map[string]bool
	pendingWorkflows map[string]bool

	// pending workflow tasks not yet picked up
	workflows chan *task.Workflow

	// pending activity tasks
	activities chan *task.Activity

	log *log.Logger
}

func NewMemoryBackend() backend.Backend {
	return &memoryBackend{
		mu:        sync.Mutex{},
		instances: make(map[string]*workflowState),

		// lockedWorkflows are currently in progress by a worker
		lockedWorkflows: make(map[string]bool),
		// pendingWorkflows have pending events
		pendingWorkflows: make(map[string]bool),

		// Queue of pending workflow instances
		workflows: make(chan *task.Workflow, 100),
		// Queue of pending activity instances
		activities: make(chan *task.Activity, 100),

		log: log.New(io.Discard, "mb", log.LstdFlags),
		// log: log.New(os.Stderr, "[mb]\t", log.Lmsgprefix|log.Ltime),
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
		History:   []history.HistoryEvent{},
		NewEvents: []history.HistoryEvent{m.HistoryEvent},
		Created:   time.Now().UTC(),
	}

	mb.instances[m.WorkflowInstance.GetInstanceID()] = &state

	mb.queueWorkflowTask(m.WorkflowInstance)

	return nil
}

func (mb *memoryBackend) SignalWorkflow(ctx context.Context, wfi core.WorkflowInstance, event history.HistoryEvent) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	instance, ok := mb.instances[wfi.GetInstanceID()]
	if !ok {
		return errors.New("workflow instance does not exist")
	}

	instance.NewEvents = append(instance.NewEvents, event)

	mb.queueWorkflowTask(wfi)

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
		delete(mb.pendingWorkflows, t.WorkflowInstance.GetInstanceID())

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

	instance := mb.instances[wfi.GetInstanceID()]

	// Remove handled events from instance
	handled := make(map[string]bool)
	for _, event := range t.NewEvents {
		handled[event.ID] = true
	}

	i := 0
	for _, event := range instance.NewEvents {
		if !handled[event.ID] {
			instance.NewEvents[i] = event
			i++
		} else {
			// Event handled, add to history
			instance.History = append(instance.History, event)
		}
	}
	instance.NewEvents = instance.NewEvents[:i]

	// Queue activities
	workflowCompleted := false

	mb.log.Println("New events:", len(newEvents))

	for _, event := range newEvents {
		mb.log.Println("\tEvent:", event.EventType)

		switch event.EventType {
		case history.HistoryEventType_ActivityScheduled:
			mb.activities <- &task.Activity{
				WorkflowInstance: wfi,
				ID:               uuid.NewString(),
				Event:            event,
			}
		case history.HistoryEventType_TimerScheduled:
			a := event.Attributes.(*history.TimerScheduledAttributes)
			// Schedule a timer to put the workflow back into the workflow queue when it fires
			go func() {
				delay := a.At.Sub(time.Now().UTC())
				<-time.After(delay)

				mb.mu.Lock()
				defer mb.mu.Unlock()

				mb.queueWorkflowTask(wfi)
			}()
		case history.HistoryEventType_WorkflowExecutionFinished:
			workflowCompleted = true
		}

		instance.NewEvents = append(instance.NewEvents, event)
	}

	// Unlock instance
	delete(mb.lockedWorkflows, wfi.GetInstanceID())

	if !workflowCompleted {
		// New events from this checkpoint or added while this was locked
		hasNewEvents := len(instance.NewEvents) > 0
		if hasNewEvents {
			mb.queueWorkflowTask(wfi)
		}
	} else {
		instance.status = workflowStatusCompleted
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
		return errors.New("workflow instance does not exist")
	}

	if instance.status == workflowStatusCompleted {
		mb.log.Println("Workflow already completed, ignoring activity result")
	} else {
		instance.NewEvents = append(instance.NewEvents, event)

		// Workflow is not currently locked, mark as ready to be picked up
		if _, ok := mb.lockedWorkflows[wfi.GetInstanceID()]; !ok {
			mb.queueWorkflowTask(wfi)
		}
	}

	return nil
}

func (mb *memoryBackend) queueWorkflowTask(wfi core.WorkflowInstance) {
	instance, ok := mb.instances[wfi.GetInstanceID()]
	if !ok {
		panic("could not find workflow instance")
	}

	// Return visible events to worker
	mb.log.Println("Events to worker:")
	newEvents := make([]history.HistoryEvent, 0)
	for _, event := range instance.NewEvents {
		if event.VisibleAt == nil || event.VisibleAt.Before(time.Now().UTC()) {
			mb.log.Println("\tEvent:", event.EventType)
			newEvents = append(newEvents, event)
		}
	}

	if len(newEvents) == 0 {
		mb.log.Println("no events")
		return
	}

	// Prevent multiple tasks for the same workflow
	if mb.pendingWorkflows[wfi.GetInstanceID()] {
		return
	}

	mb.pendingWorkflows[wfi.GetInstanceID()] = true

	// Add task to queue
	mb.workflows <- &task.Workflow{
		WorkflowInstance: wfi,
		History:          instance.History,
		NewEvents:        newEvents,
	}
}
