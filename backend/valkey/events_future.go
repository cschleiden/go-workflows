package valkey

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/valkey-io/valkey-glide/go/v2/options"
)

func scheduleFutureEvents(ctx context.Context, vb *valkeyBackend) error {
	now := time.Now().UnixMilli()
	nowStr := strconv.FormatInt(now, 10)
	_, err := vb.client.InvokeScriptWithOptions(ctx, futureEventsScript, options.ScriptOptions{
		Keys: []string{
			vb.keys.futureEventsKey(),
		},
		Args: []string{nowStr, vb.keys.prefix},
	})

	if err != nil {
		return fmt.Errorf("checking future events: %w", err)
	}

	return nil
}
