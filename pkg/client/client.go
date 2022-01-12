package client

import (
	"context"

	a "github.com/cschleiden/go-dt/internal/args"
	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/fn"
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

	CancelWorkflowInstance(ctx context.Context, wfi core.WorkflowInstance) error

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
	inputs, err := a.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert arguments")
	}

	startedEvent := history.NewHistoryEvent(
		history.EventType_WorkflowExecutionStarted,
		-1,
		&history.ExecutionStartedAttributes{
			Name:   fn.Name(wf),
			Inputs: inputs,
		})

	wfi := core.NewWorkflowInstance(options.InstanceID, uuid.NewString())

	startMessage := &core.WorkflowEvent{
		WorkflowInstance: wfi,
		HistoryEvent:     startedEvent,
	}

	if err := c.backend.CreateWorkflowInstance(ctx, *startMessage); err != nil {
		return nil, errors.Wrap(err, "could not create workflow instance")
	}

	return wfi, nil
}

func (c *client) CancelWorkflowInstance(ctx context.Context, wfi core.WorkflowInstance) error {
	// TODO: Call backend
	return nil
}

func (c *client) SignalWorkflow(ctx context.Context, wfi core.WorkflowInstance, name string, arg interface{}) error {
	input, err := converter.DefaultConverter.To(arg)
	if err != nil {
		return errors.Wrap(err, "could not convert arguments")
	}

	event := history.NewHistoryEvent(
		history.EventType_SignalReceived,
		-1,
		&history.SignalReceivedAttributes{
			Name: name,
			Arg:  input,
		},
	)

	return c.backend.SignalWorkflow(ctx, wfi, event)
}
