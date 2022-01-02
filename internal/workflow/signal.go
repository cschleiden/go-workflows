package workflow

import (
	"context"

	"github.com/cschleiden/go-dt/internal/sync"
)

func NewSignalChannel(ctx context.Context, name string) sync.Channel {
	wfState := getWfState(ctx)

	cs := sync.NewBufferedChannel(10_000)
	wfState.signalChannels[name] = cs

	return cs
}
