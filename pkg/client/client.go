package client

import (
	"context"

	"github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/converter"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type WorkflowInstanceOptions struct {
	InstanceID string
}
type TaskHubClient interface {
	CreateWorkflowInstance(ctx context.Context, options WorkflowInstanceOptions, wf workflow.Workflow, args ...interface{}) (core.WorkflowInstance, error)
}

type taskHubClient struct {
	backend backend.Backend
}

func NewTaskHubClient(backend backend.Backend) TaskHubClient {
	return &taskHubClient{
		backend: backend,
	}
}

func (c *taskHubClient) CreateWorkflowInstance(ctx context.Context, options WorkflowInstanceOptions, wf workflow.Workflow, args ...interface{}) (core.WorkflowInstance, error) {
	instanceID := options.InstanceID
	executionID := uuid.NewString()

	inputs, err := encodeArgs(converter.DefaultConverter, args...)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert arguments")
	}

	startedEvent := history.NewHistoryEvent(
		history.HistoryEventType_WorkflowExecutionStarted,
		-1,
		&history.ExecutionStartedAttributes{
			Name:   "workflow name TODO",
			Inputs: inputs,
		})

	wfi := core.NewWorkflowInstance(instanceID, executionID)

	startMessage := &core.TaskMessage{
		WorkflowInstance: wfi,
		HistoryEvent:     startedEvent,
	}

	if err := c.backend.CreateWorkflowInstance(ctx, *startMessage); err != nil {
		return nil, errors.Wrap(err, "could not create workflow instance")
	}

	return wfi, nil
}

func encodeArgs(c converter.Converter, args ...interface{}) ([][]byte, error) {
	eas := make([][]byte, len(args))

	for _, a := range args {
		ea, err := c.To(a)
		if err != nil {
			return nil, errors.Wrap(err, "could not encode arg")
		}

		eas = append(eas, ea)
	}

	return eas, nil
}
