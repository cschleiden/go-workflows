package redis

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/ticctech/go-workflows/internal/history"
	"github.com/ticctech/go-workflows/internal/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (rb *redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	instanceState, err := readInstance(ctx, rb.rdb, instanceID)
	if err != nil {
		return err
	}

	ctx = tracing.UnmarshalSpan(ctx, instanceState.Metadata)
	a := event.Attributes.(*history.SignalReceivedAttributes)
	_, span := rb.Tracer().Start(ctx, fmt.Sprintf("SignalWorkflow: %s", a.Name), trace.WithAttributes(
		attribute.String(tracing.WorkflowInstanceID, instanceID),
		attribute.String("signal.name", event.Attributes.(*history.SignalReceivedAttributes).Name),
	))
	defer span.End()

	if _, err = rb.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		if err := rb.addWorkflowInstanceEventP(ctx, p, instanceState.Instance, &event); err != nil {
			return fmt.Errorf("adding event to stream: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
