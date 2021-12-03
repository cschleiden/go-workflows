package client

import (
	"context"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/workflow"
)

type TaskHubClient interface {
	StartWorkflow(context.Context, workflow.Workflow) error
}

type taskHubClient struct {
	backend backend.Backend
}

func NewTaskHubClient(backend backend.Backend) TaskHubClient {
	return &taskHubClient{
		backend: backend,
	}
}

func (c *taskHubClient) StartWorkflow(_ context.Context, wf workflow.Workflow) error {
	// TODO: Dispatch workflow

	panic("not implemented") // TODO: Implement
}
