package redis

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

type queue struct {
	item string
	keys *keys
	rdb  redis.UniversalClient
}

func newQueue(rdb redis.UniversalClient, item string) *queue {
	return &queue{
		item,
		queueKeys(item),
		rdb,
	}
}

func (q *queue) Enqueue(ctx context.Context, id string) error {
	return q.rdb.LPush(ctx, q.keys.queue, id).Err()
}

func (q *queue) Dequeue(ctx context.Context, lease, timeout time.Duration) (*string, error) {
	cmd := q.rdb.BLMove(ctx, q.keys.queue, q.keys.processing, "RIGHT", "LEFT", timeout)
	id, err := cmd.Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}

		return nil, errors.Wrap(err, "could not get queue item")
	}

	// TESTING: Simulate failure
	if q.item == "activities" {
		return nil, nil
	}

	lockTimeout := time.Now().Add(lease).Unix()
	if q.rdb.ZAdd(ctx, q.keys.lease, &redis.Z{
		Score:  float64(lockTimeout),
		Member: id,
	}).Err() != nil {
		return nil, errors.Wrap(err, "could not store lease for queue item")
	}

	return &id, nil
}

func (q *queue) Extend(ctx context.Context, id string, lease time.Duration) error {
	lockTimeout := time.Now().Add(lease).Unix()
	if err := q.rdb.ZAdd(ctx, q.keys.lease, &redis.Z{
		Score:  float64(lockTimeout),
		Member: id,
	}).Err(); err != nil {
		return errors.Wrap(err, "could not store lease for queue item")
	}

	return nil
}

func (q *queue) Complete(ctx context.Context, id string) error {
	if err := q.rdb.LRem(ctx, q.keys.processing, 1, id).Err(); err != nil {
		return errors.Wrap(err, "could not remove queue item from processing queue")
	}

	if err := q.rdb.ZRem(ctx, q.keys.lease, id).Err(); err != nil {
		return errors.Wrap(err, "could not remove queue item from lease set")
	}

	return nil
}

// Check all items in the processing queue, if one
// - doesn't have a lease, or
// - the lease is expired
// then remove it from processing, remove the lease (if it exists), and add back to
// queued.
// - KEYS[1] = processing LIST
// - KEYS[2] = lease ZSET
// - KEYS[3] = queued LIST
// - ARGV[1] = current timestamp
var recoverCmd = redis.NewScript(`
local res = {}
local ids = redis.call("LRANGE", KEYS[1], 0, -1)
if next(ids) == nil then
	return res
end

local leases = redis.call("ZMSCORE", KEYS[2], unpack(ids))
for i, id in ipairs(ids) do
	local has_lease = next(leases) ~= nil and leases[i] ~= nil
	if not has_lease or leases[i] < ARGV[1] then
		redis.call("LPUSH", KEYS[3], id)

		if has_lease then
			redis.call("ZREM", KEYS[2], id)
		end

		table.insert(res, id)
	end
end
return res
`)

func (q *queue) Recover(ctx context.Context) ([]string, error) {
	now := time.Now().Unix()
	res, err := recoverCmd.Run(ctx, q.rdb, []string{q.keys.processing, q.keys.lease, q.keys.queue}, now).Result()
	if err != nil {
		return nil, errors.Wrap(err, "could not recover queue")
	}

	arr := res.([]interface{})
	ids := make([]string, len(arr))
	for i, v := range arr {
		ids[i] = v.(string)
	}

	return ids, nil
}
