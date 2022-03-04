package client

import (
	"context"
	"time"

	a "github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/pkg/backend"
	"github.com/cschleiden/go-workflows/pkg/core"
	"github.com/cschleiden/go-workflows/pkg/history"
	"github.com/cschleiden/go-workflows/pkg/workflow"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type WorkflowInstanceOptions struct {
	InstanceID string
}

type Client interface {
	CreateWorkflowInstance(ctx context.Context, options WorkflowInstanceOptions, wf workflow.Workflow, args ...interface{}) (core.WorkflowInstance, error)

	CancelWorkflowInstance(ctx context.Context, instance core.WorkflowInstance) error

	SignalWorkflow(ctx context.Context, instanceID string, name string, arg interface{}) error
}

type client struct {
	backend backend.Backend
}

func New(backend backend.Backend) Client {
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
		time.Now(),
		history.EventType_WorkflowExecutionStarted,
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

func (c *client) CancelWorkflowInstance(ctx context.Context, instance core.WorkflowInstance) error {
	return c.backend.CancelWorkflowInstance(ctx, instance)
}

func (c *client) SignalWorkflow(ctx context.Context, instanceID string, name string, arg interface{}) error {
	input, err := converter.DefaultConverter.To(arg)
	if err != nil {
		return errors.Wrap(err, "could not convert arguments")
	}

	event := history.NewHistoryEvent(
		time.Now(),
		history.EventType_SignalReceived,
		&history.SignalReceivedAttributes{
			Name: name,
			Arg:  input,
		},
	)

	return c.backend.SignalWorkflow(ctx, instanceID, event)
}
