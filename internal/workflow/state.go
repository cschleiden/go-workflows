package workflow

import (
	"time"

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
	time           time.Time
}

func newWorkflowState() *workflowState {
	return &workflowState{
		commands:       []command.Command{},
		eventID:        0,
		pendingFutures: map[int]sync.Future{},
		signalChannels: make(map[string]sync.Channel),
	}
}

func getWfState(ctx sync.Context) *workflowState {
	return ctx.Value(workflowCtxKey).(*workflowState)
}

func withWfState(ctx sync.Context, wfState *workflowState) sync.Context {
	return sync.WithValue(ctx, workflowCtxKey, wfState)
}

func (wf *workflowState) addCommand(cmd command.Command) {
	wf.commands = append(wf.commands, cmd)
}

func (wf *workflowState) removeCommandByEventID(eventID int) *command.Command {
	for i, c := range wf.commands {
		if c.ID == eventID {
			wf.commands = append(wf.commands[:i], wf.commands[i+1:]...)
			return &c
		}
	}

	return nil
}

func (wf *workflowState) removeCommand(cmd command.Command) {
	for i, c := range wf.commands {
		if c == cmd {
			wf.commands = append(wf.commands[:i], wf.commands[i+1:]...)
			return
		}
	}
}

func (wf *workflowState) clearCommands() {
	wf.commands = []command.Command{}
}

func (wf *workflowState) createSignalChannel(name string) sync.Channel {
	cs := sync.NewBufferedChannel(10_000)
	wf.signalChannels[name] = cs
	return cs
}

func (wf *workflowState) getSignalChannel(name string) sync.Channel {
	cs, ok := wf.signalChannels[name]
	if ok {
		return cs
	}

	return wf.createSignalChannel(name)
}

func (wf *workflowState) setReplaying(replaying bool) {
	wf.replaying = replaying
}

func (wf *workflowState) setTime(t time.Time) {
	wf.time = t
}
