package redis

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/core"
	redis "github.com/redis/go-redis/v9"
)

// KEYS[1] - instance key
// KEYS[2] - pending events key
// KEYS[3] - history key
// KEYS[4] - payload key
// KEYS[5] - active-instance-execution key
// KEYS[6] - instances-by-creation key
// ARGV[1] - instance segment
var deleteInstanceCmd = redis.NewScript(
	`redis.call("DEL", KEYS[1], KEYS[2], KEYS[3], KEYS[4], KEYS[5])
	return redis.call("ZREM", KEYS[6], ARGV[1])`)

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
