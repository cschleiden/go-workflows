package redis

import (
	"context"
	"strconv"
	"time"

	"github.com/cschleiden/go-workflows/core"
)

func (rb *redisBackend) setWorkflowInstanceExpiration(ctx context.Context, instance *core.WorkflowInstance, expiration time.Duration) error {
	now := time.Now().UnixMilli()
	nowStr := strconv.FormatInt(now, 10)

	exp := time.Now().Add(expiration).UnixMilli()
	expStr := strconv.FormatInt(exp, 10)

	return expireWorkflowInstanceCmd.Run(ctx, rb.rdb, []string{
		rb.keys.instancesByCreation(),
		rb.keys.instancesExpiring(),
		rb.keys.instanceKey(instance),
		rb.keys.pendingEventsKey(instance),
		rb.keys.historyKey(instance),
		rb.keys.payloadKey(instance),
	},
		nowStr,
		expiration.Seconds(),
		expStr,
		instanceSegment(instance),
	).Err()
}
