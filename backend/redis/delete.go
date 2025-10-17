package redis

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/core"
)

// deleteInstance deletes an instance from Redis. It does not attempt to remove any future events or pending
// workflow tasks. It's assumed that the instance is in the finished state.
//
// Note: might want to revisit this in the future if we want to support removing hung instances.
func (rb *redisBackend) deleteInstance(ctx context.Context, instance *core.WorkflowInstance) error {
	if err := deleteInstanceCmd.Run(ctx, rb.rdb, []string{
		rb.keys.instanceKey(instance),
		rb.keys.pendingEventsKey(instance),
		rb.keys.historyKey(instance),
		rb.keys.payloadKey(instance),
		rb.keys.activeInstanceExecutionKey(instance.InstanceID),
		rb.keys.instancesByCreation(),
	}, instanceSegment(instance)).Err(); err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	return nil
}
