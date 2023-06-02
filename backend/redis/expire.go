package redis

import (
	"context"
	"strconv"
	"time"

	"github.com/cschleiden/go-workflows/internal/core"
	redis "github.com/redis/go-redis/v9"
)

// We can't have events for redis..we do not want to do it in-process..

// Set the given expiration time on all keys passed in
// KEYS[1] - instances-by-creation key
// KEYS[2] - instances-expiring key
// KEYS[3] - instance key
// KEYS[4] - pending events key
// KEYS[5] - history key
// ARGV[1] - current timestamp
// ARGV[2] - expiration time in seconds
// ARGV[3] - expiration timestamp in unix milliseconds
// ARGV[4] - instance segment
var expireCmd = redis.NewScript(
	`-- Find instances which have already expired and remove from the index set
	local expiredInstances = redis.call("ZRANGE", KEYS[2], "-inf", ARGV[1], "BYSCORE")
	for i = 1, #expiredInstances do
		local instanceSegment = expiredInstances[i]
		redis.call("ZREM", KEYS[1], instanceSegment) -- index set
		redis.call("ZREM", KEYS[2], instanceSegment) -- expiration set
	end

	-- Add expiration time for future cleanup
	redis.call("ZADD", KEYS[2], ARGV[3], ARGV[4])

	-- Set expiration on all keys
	for i = 3, #KEYS do
		redis.call("EXPIRE", KEYS[i], ARGV[2])
	end

	return 0
	`,
)

func setWorkflowInstanceExpiration(ctx context.Context, rdb redis.UniversalClient, instance *core.WorkflowInstance, expiration time.Duration) error {
	now := time.Now().UnixMilli()
	nowStr := strconv.FormatInt(now, 10)

	exp := time.Now().Add(expiration).UnixMilli()
	expStr := strconv.FormatInt(exp, 10)

	return expireCmd.Run(ctx, rdb, []string{
		instancesByCreation(),
		instancesExpiring(),
		instanceKey(instance),
		pendingEventsKey(instance),
		historyKey(instance),
	},
		nowStr,
		expiration.Seconds(),
		expStr,
		instanceSegment(instance),
	).Err()
}
