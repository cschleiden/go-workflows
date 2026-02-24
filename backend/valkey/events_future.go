package valkey

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

func scheduleFutureEvents(ctx context.Context, vb *valkeyBackend) error {
	now := time.Now().UnixMilli()
	nowStr := strconv.FormatInt(now, 10)
	err := futureEventsScript.Exec(ctx, vb.client, []string{vb.keys.futureEventsKey()}, []string{nowStr, vb.keys.prefix}).Error()

	if err != nil {
		return fmt.Errorf("checking future events: %w", err)
	}

	return nil
}
