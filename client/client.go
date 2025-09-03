package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metrics"
	"github.com/cschleiden/go-workflows/core"
	a "github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/log"
	"github.com/cschleiden/go-workflows/internal/metrickeys"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/cschleiden/go-workflows/workflow"
)

// ErrWorkflowCanceled is returned when a workflow was already canceled.
var ErrWorkflowCanceled = errors.New("workflow canceled")

// ErrWorkflowTerminated is returned when a workflow was already terminated.
var ErrWorkflowTerminated = errors.New("workflow terminated")

type WorkflowInstanceOptions struct {
	// Queue is the queue the workflow instance will be created in. Must be a valid queue
	// for the given backend. If not set, will default to the default queue
	Queue workflow.Queue

	InstanceID string
}

type Client struct {
	backend backend.Backend
	clock   clock.Clock
}

// New creates a new client for the given backend.
func New(backend backend.Backend) *Client {
	return &Client{
		backend: backend,
		clock:   clock.New(),
	}
}

// CreateWorkflowInstance creates a new workflow instance of the given workflow.
func (c *Client) CreateWorkflowInstance(ctx context.Context, options WorkflowInstanceOptions, wf workflow.Workflow, args ...any) (*workflow.Instance, error) {
	var workflowName string

	if name, ok := wf.(string); ok {
		workflowName = name
	} else {
		workflowName = fn.Name(wf)

		// Check arguments if actual workflow function given here
		if err := a.ParamsMatch(wf, args...); err != nil {
			return nil, err
		}
	}

	inputs, err := a.ArgsToInputs(c.backend.Options().Converter, args...)
	if err != nil {
		return nil, fmt.Errorf("converting arguments: %w", err)
	}

	if options.InstanceID == "" {
		return nil, errors.New("InstanceID must be set")
	}

	if options.Queue == "" {
		options.Queue = workflow.QueueDefault
	}

	wfi := core.NewWorkflowInstance(options.InstanceID, uuid.NewString())
	metadata := &workflow.Metadata{}

	// Span for creating the workflow instance
	ctx, span := c.backend.Tracer().Start(ctx, "CreateWorkflowInstance", trace.WithAttributes(
		attribute.String(log.InstanceIDKey, wfi.InstanceID),
		attribute.String(log.ExecutionIDKey, wfi.ExecutionID),
		attribute.String(log.WorkflowNameKey, workflowName),
	), trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()

	// Inject state from any propagators
	for _, propagator := range c.backend.Options().ContextPropagators {
		if err := propagator.Inject(ctx, metadata); err != nil {
			return nil, fmt.Errorf("injecting context to propagate: %w", err)
		}
	}

	workflowSpanID := tracing.GetNewSpanID(c.backend.Tracer())

	startedEvent := history.NewPendingEvent(
		c.clock.Now(),
		history.EventType_WorkflowExecutionStarted,
		&history.ExecutionStartedAttributes{
			Queue:          options.Queue,
			Metadata:       metadata,
			Name:           workflowName,
			Inputs:         inputs,
			WorkflowSpanID: workflowSpanID,
		})

	if err := c.backend.CreateWorkflowInstance(ctx, wfi, startedEvent); err != nil {
		return nil, fmt.Errorf("creating workflow instance: %w", err)
	}

	c.backend.Options().Logger.Debug(
		"Created workflow instance",
		log.InstanceIDKey, wfi.InstanceID,
		log.ExecutionIDKey, wfi.ExecutionID,
		log.WorkflowNameKey, workflowName,
	)

	c.backend.Metrics().Counter(metrickeys.WorkflowInstanceCreated, metrics.Tags{}, 1)

	return wfi, nil
}

// CancelWorkflowInstance cancels a running workflow instance.
func (c *Client) CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance) error {
	ctx, span := c.backend.Tracer().Start(ctx, "CancelWorkflowInstance", trace.WithAttributes(
		attribute.String(log.InstanceIDKey, instance.InstanceID),
	))
	defer span.End()

	cancellationEvent := history.NewWorkflowCancellationEvent(time.Now())
	return c.backend.CancelWorkflowInstance(ctx, instance, cancellationEvent)
}

// SignalWorkflow signals a running workflow instance.
func (c *Client) SignalWorkflow(ctx context.Context, instanceID string, name string, arg any) error {
	ctx, span := c.backend.Tracer().Start(ctx, "SignalWorkflow", trace.WithAttributes(
		attribute.String(log.InstanceIDKey, instanceID),
		attribute.String(log.SignalNameKey, name),
	))
	defer span.End()

	input, err := c.backend.Options().Converter.To(arg)
	if err != nil {
		return fmt.Errorf("converting arguments: %w", err)
	}

	signalEvent := history.NewPendingEvent(
		c.clock.Now(),
		history.EventType_SignalReceived,
		&history.SignalReceivedAttributes{
			Name: name,
			Arg:  input,
		},
	)

	err = c.backend.SignalWorkflow(ctx, instanceID, signalEvent)
	if err != nil {
		span.RecordError(err)
		return err
	}

	c.backend.Options().Logger.Debug("Signaled workflow instance", log.InstanceIDKey, instanceID)

	return nil
}

// GetWorkflowInstanceState returns the current state of the given workflow instance
func (c *Client) GetWorkflowInstanceState(ctx context.Context, instance *workflow.Instance) (core.WorkflowInstanceState, error) {
	return c.backend.GetWorkflowInstanceState(ctx, instance)
}

// WaitForWorkflowInstance waits for the given workflow instance to finish or until the given timeout has expired.
func (c *Client) WaitForWorkflowInstance(ctx context.Context, instance *workflow.Instance, timeout time.Duration) error {
	if timeout == 0 {
		timeout = time.Second * 20
	}

	ctx, span := c.backend.Tracer().Start(ctx, "WaitForWorkflowInstance", trace.WithAttributes(
		attribute.String(log.InstanceIDKey, instance.InstanceID),
	))
	defer span.End()

	b := backoff.ExponentialBackOff{
		InitialInterval:     time.Millisecond * 1,
		MaxInterval:         time.Second * 1,
		Multiplier:          1.5,
		RandomizationFactor: 0.5,
		MaxElapsedTime:      timeout,
		Stop:                backoff.Stop,
		Clock:               c.clock,
	}
	b.Reset()

	ticker := backoff.NewTicker(&b)
	defer ticker.Stop()

	for range ticker.C {
		s, err := c.backend.GetWorkflowInstanceState(ctx, instance)
		if err != nil {
			return fmt.Errorf("getting workflow state: %w", err)
		}

		if s == core.WorkflowInstanceStateFinished || s == core.WorkflowInstanceStateContinuedAsNew {
			return nil
		}
	}

	return errors.New("workflow did not finish in specified timeout")
}

// GetWorkflowResult gets the workflow result for the given workflow result. It first waits for the workflow to finish or until
// the given timeout has expired.
func GetWorkflowResult[T any](ctx context.Context, c *Client, instance *workflow.Instance, timeout time.Duration) (T, error) {
	b := c.backend

	ctx, span := b.Tracer().Start(ctx, "GetWorkflowResult", trace.WithAttributes(
		attribute.String(log.InstanceIDKey, instance.InstanceID),
	))
	defer span.End()

	if err := c.WaitForWorkflowInstance(ctx, instance, timeout); err != nil {
		return *new(T), fmt.Errorf("workflow did not finish in time: %w", err)
	}

	h, err := b.GetWorkflowInstanceHistory(ctx, instance, nil) // future: could optimize this by retriving only the very last entry in the history
	if err != nil {
		return *new(T), fmt.Errorf("getting workflow history: %w", err)
	}

	// Iterate over history backwards
	for i := len(h) - 1; i >= 0; i-- {
		event := h[i]
		switch event.Type {
		case history.EventType_WorkflowExecutionFinished:
			a := event.Attributes.(*history.ExecutionCompletedAttributes)
			if a.Error != nil {
				return *new(T), workflowerrors.ToError(a.Error)
			}

			var r T
			if err := b.Options().Converter.From(a.Result, &r); err != nil {
				return *new(T), fmt.Errorf("converting result: %w", err)
			}

			return r, nil

		case history.EventType_WorkflowExecutionContinuedAsNew:
			a := event.Attributes.(*history.ExecutionContinuedAsNewAttributes)

			var r T
			if err := b.Options().Converter.From(a.Result, &r); err != nil {
				return *new(T), fmt.Errorf("converting result: %w", err)
			}

			return r, nil

		case history.EventType_WorkflowExecutionCanceled:
			return *new(T), ErrWorkflowCanceled

		case history.EventType_WorkflowExecutionTerminated:
			return *new(T), ErrWorkflowTerminated
		}
	}

	return *new(T), errors.New("workflow finished, but could not find result event")
}

// RemoveWorkflowInstance removes the given workflow instance from the backend.
//
// Instance needs to be in a completed state.
func (c *Client) RemoveWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance) error {
	ctx, span := c.backend.Tracer().Start(ctx, "RemoveWorkflowInstance", trace.WithAttributes(
		attribute.String(log.InstanceIDKey, instance.InstanceID),
	))
	defer span.End()

	return c.backend.RemoveWorkflowInstance(ctx, instance)
}

// RemoveWorkflowInstances removes completed workflow instances from the backend.
func (c *Client) RemoveWorkflowInstances(ctx context.Context, options ...backend.RemovalOption) error {
	ctx, span := c.backend.Tracer().Start(ctx, "RemoveWorkflowInstances")
	defer span.End()

	return c.backend.RemoveWorkflowInstances(ctx, options...)
}
