package workflow

import (
	"github.com/cschleiden/go-workflows/internal/sync"
)

func NewSignalChannel(ctx sync.Context, name string) sync.Channel {
	wfState := getWfState(ctx)

	return wfState.getSignalChannel(name)
}
