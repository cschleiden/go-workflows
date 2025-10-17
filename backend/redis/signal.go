package redis

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/workflow"
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

	eventData, payload, err := marshalEvent(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	queue := workflow.Queue(instanceState.Queue)
	queueKeys := rb.workflowQueue.Keys(queue)

	keys := []string{
		rb.keys.payloadKey(instanceState.Instance),
		rb.keys.pendingEventsKey(instanceState.Instance),
		queueKeys.SetKey,
		queueKeys.StreamKey,
	}

	args := []any{
		event.ID,
		eventData,
		payload,
		instanceSegment(instanceState.Instance),
	}

	// Execute the Lua script
	_, err = signalWorkflowCmd.Run(ctx, rb.rdb, keys, args...).Result()
	if err != nil {
		return fmt.Errorf("signaling workflow: %w", err)
	}

	return nil
}
