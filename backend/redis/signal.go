package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

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

	if _, err = rb.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		if err := rb.addWorkflowInstanceEventP(ctx, p, workflow.Queue(instanceState.Queue), instanceState.Instance, event); err != nil {
			return fmt.Errorf("adding event to stream: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
