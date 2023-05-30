package redis

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/cschleiden/go-workflows/log"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (rb *redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event *history.Event) error {
	instanceState, err := readInstance(ctx, rb.rdb, instanceID)
	if err != nil {
		return err
	}

	ctx, err = (&tracing.TracingContextPropagator{}).Extract(ctx, instanceState.Metadata)
	if err != nil {
		rb.Logger().Error("extracting tracing context", log.ErrorKey, err)
	}

	a := event.Attributes.(*history.SignalReceivedAttributes)
	ctx, span := rb.Tracer().Start(ctx, fmt.Sprintf("SignalWorkflow: %s", a.Name), trace.WithAttributes(
		attribute.String(log.InstanceIDKey, instanceID),
		attribute.String(log.SignalNameKey, event.Attributes.(*history.SignalReceivedAttributes).Name),
	))
	defer span.End()

	if _, err = rb.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		if err := rb.addWorkflowInstanceEventP(ctx, p, instanceState.Instance, event); err != nil {
			return fmt.Errorf("adding event to stream: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
