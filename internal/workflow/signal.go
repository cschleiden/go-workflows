package workflow

import (
	"github.com/cschleiden/go-dt/internal/sync"
)

func NewSignalChannel(ctx sync.Context, name string) sync.Channel {
	wfState := getWfState(ctx)

	cs := sync.NewBufferedChannel(10_000)
	wfState.signalChannels[name] = cs

	return cs
}
