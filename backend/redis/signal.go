package redis

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/backend/redis/taskqueue"
	"github.com/cschleiden/go-workflows/internal/history"
)

func (rb *redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	_, err := readInstance(ctx, rb.rdb, instanceID)
	if err != nil {
		return err
	}

	msgID, err := addEventToStream(ctx, rb.rdb, pendingEventsKey(instanceID), &event)
	if err != nil {
		return fmt.Errorf("adding event to stream: %w", err)
	}

	if _, err := rb.workflowQueue.Enqueue(ctx, instanceID, &workflowTaskData{
		LastPendingEventMessageID: *msgID,
	}); err != nil {
		if err != taskqueue.ErrTaskAlreadyInQueue {
			return fmt.Errorf("queueing workflow task: %w", err)
		}
	}

	return nil
}
