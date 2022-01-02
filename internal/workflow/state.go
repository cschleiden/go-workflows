package workflow

import (
	"context"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/sync"
)

type key int

var workflowCtxKey key

type workflowState struct {
	eventID        int
	commands       []command.Command
	pendingFutures map[int]sync.Future
	signalChannels map[string]sync.Channel
	replaying      bool
}

func newWorkflowState() *workflowState {
	return &workflowState{
		commands:       []command.Command{},
		eventID:        0,
		pendingFutures: map[int]sync.Future{},
		signalChannels: make(map[string]sync.Channel),
	}
}

func getWfState(ctx context.Context) *workflowState {
	return ctx.Value(workflowCtxKey).(*workflowState)
}

func withWfState(ctx context.Context, wfState *workflowState) context.Context {
	return context.WithValue(ctx, workflowCtxKey, wfState)
}

func (wf *workflowState) addCommand(cmd command.Command) {
	wf.commands = append(wf.commands, cmd)
}

func (wf *workflowState) getSignalChannel(name string) sync.Channel {
	cs, ok := wf.signalChannels[name]
	if !ok {
		panic("signal channel not found")
	}

	return cs
}

func (wf *workflowState) setReplaying(replaying bool) {
	wf.replaying = replaying
}
