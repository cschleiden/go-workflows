package client

import (
	"context"

	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/workflow"
	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type WorkflowInstanceOptions struct {
	InstanceID string
}

type Client interface {
	CreateWorkflowInstance(ctx context.Context, options WorkflowInstanceOptions, wf workflow.Workflow, args ...interface{}) (core.WorkflowInstance, error)

	SignalWorkflow(ctx context.Context, wfi core.WorkflowInstance, name string, arg interface{}) error
}

type client struct {
	backend backend.Backend
}

func NewClient(backend backend.Backend) Client {
	return &client{
		backend: backend,
	}
}

func (c *client) CreateWorkflowInstance(ctx context.Context, options WorkflowInstanceOptions, wf workflow.Workflow, args ...interface{}) (core.WorkflowInstance, error) {
	inputs, err := converter.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert arguments")
	}

	startedEvent := history.NewHistoryEvent(
		history.HistoryEventType_WorkflowExecutionStarted,
		-1,
		&history.ExecutionStartedAttributes{
			Name:   "wf1",
			Inputs: inputs,
		})

	instanceID := options.InstanceID
	executionID := uuid.NewString()
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

func (c *client) SignalWorkflow(ctx context.Context, wfi core.WorkflowInstance, name string, arg interface{}) error {
	input, err := converter.DefaultConverter.To(arg)
	if err != nil {
		return errors.Wrap(err, "could not convert arguments")
	}

	event := history.NewHistoryEvent(
		history.HistoryEventType_SignalReceived,
		-1,
		history.SignalReceivedAttributes{
			Name: name,
			Arg:  input,
		},
	)

	return c.backend.SignalWorkflow(ctx, wfi, event)
}
