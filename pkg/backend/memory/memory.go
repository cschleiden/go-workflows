package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/history"
)

// TODO: This will have to move somewhere else
type workflowState struct {
	Name string

	Created time.Time
}

type memoryBackend struct {
	instanceStore map[string]map[string]workflowState

	mu sync.Mutex

	workflowQueue Queue
	activityQueue Queue
}

func NewMemoryBackend() backend.Backend {
	return &memoryBackend{
		instanceStore: make(map[string]map[string]workflowState),
		mu:            sync.Mutex{},

		workflowQueue: NewQueue(),
		activityQueue: NewQueue(),
	}
}

func (mb *memoryBackend) CreateWorkflowInstance(ctx context.Context, m core.TaskMessage) error {
	attrs, ok := m.HistoryEvent.Attributes.(*history.ExecutionStartedAttributes)
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
	mb.workflowQueue.Push(m)

	return nil
}

func (mb *memoryBackend) SendWorkflowInstanceMessage(_ context.Context, _ core.TaskMessage) error {
	panic("not implemented") // TODO: Implement
}
