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

func (b *memoryBackend) CreateWorkflowInstance(message core.TaskMessage) error {
	panic("not implemented")
}

func (b *memoryBackend) SendWorkflowInstanceMessage(message core.TaskMessage) error {
	panic("not implemented")
}

// func (b *memoryBackend) GetWorkflowTask() (backend.WorkItem, error) {
// 	panic("not implemented") // TODO: Implement
// }

// func (b *memoryBackend) GetActivityTask() (backend.WorkItem, error) {
// 	panic("not implemented") // TODO: Implement
// }
