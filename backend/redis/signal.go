package redis

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/go-redis/redis/v8"
)

func (rb *redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	_, err := readInstance(ctx, rb.rdb, instanceID)
	if err != nil {
		return err
	}

	if _, err = rb.rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
		if err := addEventToStreamP(ctx, p, pendingEventsKey(instanceID), &event); err != nil {
			return fmt.Errorf("adding event to stream: %w", err)
		}

		if err := rb.workflowQueue.Enqueue(ctx, p, instanceID, nil); err != nil {
			if err != errTaskAlreadyInQueue {
				return fmt.Errorf("queueing workflow task: %w", err)
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
