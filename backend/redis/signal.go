package redis

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/internal/log"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (rb *redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event *history.Event) error {
	// Get current execution of the instance
	instance, err := rb.readActiveInstanceExecution(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("reading active instance execution: %w", err)
	}

	if instance == nil {
		return backend.ErrInstanceNotFound
	}

	instanceState, err := readInstance(ctx, rb.rdb, rb.keys.instanceKey(instance))
	if err != nil {
		return err
	}

	ctx, err = (&tracing.TracingContextPropagator{}).Extract(ctx, instanceState.Metadata)
	if err != nil {
		rb.Options().Logger.Error("extracting tracing context", log.ErrorKey, err)
	}

	a := event.Attributes.(*history.SignalReceivedAttributes)
	ctx, span := rb.Tracer().Start(ctx, fmt.Sprintf("SignalWorkflow: %s", a.Name), trace.WithAttributes(
		attribute.String(log.InstanceIDKey, instanceID),
		attribute.String(log.SignalNameKey, event.Attributes.(*history.SignalReceivedAttributes).Name),
	))
	defer span.End()

	if _, err = rb.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		if err := rb.addWorkflowInstanceEventP(ctx, p, instanceState.Queue, instanceState.Instance, event); err != nil {
			return fmt.Errorf("adding event to stream: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
