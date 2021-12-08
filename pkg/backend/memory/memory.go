package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/google/uuid"
)

// TODO: This will have to move somewhere else
type workflowState struct {
	Name string

	Created time.Time
}

// Simple in-memory backend for development
type memoryBackend struct {
	instanceStore map[string]map[string]workflowState

	mu sync.Mutex

	// workflows not yet picked up
	workflows chan *task.Workflow

	lockedWorkflows map[string]*task.Workflow

	activities chan *task.Activity

	lockedActivities map[string]*task.Activity
}

func NewMemoryBackend() backend.Backend {
	return &memoryBackend{
		instanceStore: make(map[string]map[string]workflowState),
		mu:            sync.Mutex{},

		// Queue of unlocked workflow instances
		workflows:       make(chan *task.Workflow, 100),
		lockedWorkflows: make(map[string]*task.Workflow),

		activities:       make(chan *task.Activity, 100),
		lockedActivities: make(map[string]*task.Activity),
	}
}

func (mb *memoryBackend) CreateWorkflowInstance(ctx context.Context, m core.TaskMessage) error {
	attrs, ok := m.HistoryEvent.Attributes.(history.ExecutionStartedAttributes)
	if !ok {
		return errors.New("invalid workflow instance creation event")
	}

	mb.mu.Lock()
	defer mb.mu.Unlock()

	x, ok := mb.instanceStore[m.WorkflowInstance.GetInstanceID()]
	if !ok {
		x = make(map[string]workflowState)
		mb.instanceStore[m.WorkflowInstance.GetInstanceID()] = x
	}

	// TODO: Check for existing workflow instances
	newState := workflowState{
		Name: attrs.Name,
	}

	x[m.WorkflowInstance.GetExecutionID()] = newState

	// Add to queue
	// TODO: Check if this already exists
	mb.workflows <- &task.Workflow{
		WorkflowInstance: m.WorkflowInstance,
		History:          []history.HistoryEvent{m.HistoryEvent},
	}

	return nil
}

func (mb *memoryBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	select {
	case <-ctx.Done():
		return nil, nil

	case t := <-mb.workflows:
		mb.lockedWorkflows[t.WorkflowInstance.GetExecutionID()] = t
		return t, nil
	}
}

func (mb *memoryBackend) CompleteWorkflowTask(_ context.Context, t task.Workflow, commands []command.Command) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	_, ok := mb.lockedWorkflows[t.WorkflowInstance.GetExecutionID()]
	if !ok {
		panic("could not unlock workflow instance")
	}

	workflowComplete := false
	scheduledActivity := false

	for _, c := range commands {
		switch c.Type {
		case command.CommandType_ScheduleActivityTask:
			a := c.Attr.(command.ScheduleActivityTaskCommandAttr)
			mb.activities <- &task.Activity{
				WorkflowInstance: t.WorkflowInstance,
				ID:               uuid.NewString(),
				Event: history.NewHistoryEvent(
					history.HistoryEventType_ActivityScheduled,
					int64(c.ID),
					history.ActivityScheduledAttributes{
						Name:    a.Name,
						Version: a.Version,
						Inputs:  [][]byte{},
					},
				),
			}

			scheduledActivity = true

		case command.CommandType_CompleteWorkflow:
			// _ := c.Attr.(command.CompleteWorkflowCommandAttr)
			workflowComplete = true

		default:
			// panic("unsupported command")
		}
	}

	// Return to queue
	if workflowComplete {
		delete(mb.lockedWorkflows, t.WorkflowInstance.GetExecutionID())
	} else if !scheduledActivity {
		// Unlock workflow instance
		delete(mb.lockedWorkflows, t.WorkflowInstance.GetExecutionID())
		mb.workflows <- &t
	}

	return nil
}

func (mb *memoryBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	select {
	case <-ctx.Done():
		return nil, nil

	case t := <-mb.activities:
		mb.lockedActivities[t.ID] = t
		return t, nil
	}
}

func (mb *memoryBackend) CompleteActivityTask(_ context.Context, t task.Activity, event history.HistoryEvent) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	delete(mb.lockedActivities, t.ID)

	// Continue workflow
	wt := mb.lockedWorkflows[t.WorkflowInstance.GetExecutionID()]

	wt.History = append(wt.History, event)

	delete(mb.lockedWorkflows, t.WorkflowInstance.GetExecutionID())
	mb.workflows <- wt

	return nil
}
