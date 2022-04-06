package redis

import (
	"context"

	"github.com/cschleiden/go-workflows/internal/history"
)

func (rb *redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	// TODO: Store signal event
	// TODO: Queue workflow task

	panic("unimplemented")
}
