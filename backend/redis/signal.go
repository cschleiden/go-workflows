package redis

import (
	"context"

	"github.com/cschleiden/go-workflows/backend/redis/taskqueue"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/pkg/errors"
)

func (rb *redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	msgID, err := addEventToStream(ctx, rb.rdb, pendingEventsKey(instanceID), &event)
	if err != nil {
		return errors.Wrap(err, "could not add event to stream")
	}

	if _, err := rb.workflowQueue.Enqueue(ctx, instanceID, &workflowTaskData{
		LastPendingEventMessageID: *msgID,
	}); err != nil {
		if err != taskqueue.ErrTaskAlreadyInQueue {
			return errors.Wrap(err, "could not queue workflow task")
		}
	}

	return nil
}
