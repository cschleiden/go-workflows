package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	redis "github.com/redis/go-redis/v9"
)

func scheduleFutureEvents(ctx context.Context, rb *redisBackend) error {
	now := time.Now().UnixMilli()
	nowStr := strconv.FormatInt(now, 10)
	if _, err := futureEventsCmd.Run(ctx, rb.rdb, []string{
		rb.keys.futureEventsKey(),
	}, nowStr, rb.keys.prefix).Result(); err != nil && err != redis.Nil {
		return fmt.Errorf("checking future events: %w", err)
	}

	return nil
}
