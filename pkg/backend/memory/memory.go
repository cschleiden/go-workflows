package memory

import (
	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core"
)

type memoryBackend struct {
}

func NewMemoryBackend() backend.Backend {
	return &memoryBackend{}
}

func (b *memoryBackend) CreateWorkflowInstance(id core.WorkflowInstanceID, wf core.Workflow) error {
	panic("not implemented")
}

func (b *memoryBackend) GetWorkflowTask() (backend.WorkItem, error) {
	panic("not implemented") // TODO: Implement
}

func (b *memoryBackend) GetActivityTask() (backend.WorkItem, error) {
	panic("not implemented") // TODO: Implement
}
