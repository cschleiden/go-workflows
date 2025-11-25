package valkey

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/cschleiden/go-workflows/core"
	"github.com/valkey-io/valkey-glide/go/v2/options"
)

func (vb *valkeyBackend) setWorkflowInstanceExpiration(ctx context.Context, instance *core.WorkflowInstance, expiration time.Duration) error {
	now := time.Now().UnixMilli()
	nowStr := strconv.FormatInt(now, 10)

	exp := time.Now().Add(expiration).UnixMilli()
	expStr := strconv.FormatInt(exp, 10)

	_, err := vb.client.InvokeScriptWithOptions(ctx, expireWorkflowInstanceScript, options.ScriptOptions{
		Keys: []string{
			vb.keys.instancesByCreation(),
			vb.keys.instancesExpiring(),
			vb.keys.instanceKey(instance),
			vb.keys.pendingEventsKey(instance),
			vb.keys.historyKey(instance),
			vb.keys.payloadKey(instance),
		},
		Args: []string{
			nowStr,
			fmt.Sprintf("%.0f", expiration.Seconds()),
			expStr,
			instanceSegment(instance),
		},
	})

	return err
}
