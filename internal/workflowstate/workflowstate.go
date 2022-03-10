package workflowstate

import (
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/sync"
)

type key int

var workflowCtxKey key

type WfState struct {
	instance        core.WorkflowInstance
	scheduleEventID int
	commands        []*command.Command
	pendingFutures  map[int]sync.Future
	signalChannels  map[string]sync.Channel
	replaying       bool

	clock clock.Clock
	time  time.Time
}

func NewWorkflowState(instance core.WorkflowInstance, clock clock.Clock) *WfState {
	return &WfState{
		instance:        instance,
		commands:        []*command.Command{},
		scheduleEventID: 1,
		pendingFutures:  map[int]sync.Future{},
		signalChannels:  make(map[string]sync.Channel),
		clock:           clock,
	}
}

func WorkflowState(ctx sync.Context) *WfState {
	return ctx.Value(workflowCtxKey).(*WfState)
}

func WithWorkflowState(ctx sync.Context, wfState *WfState) sync.Context {
	return sync.WithValue(ctx, workflowCtxKey, wfState)
}

func (wf *WfState) GetNextScheduleEventID() int {
	scheduleEventID := wf.scheduleEventID
	wf.scheduleEventID++
	return scheduleEventID
}

func (wf *WfState) TrackFuture(scheduleEventID int, f sync.Future) {
	wf.pendingFutures[scheduleEventID] = f
}

func (wf *WfState) FutureByScheduleEventID(scheduleEventID int) (sync.Future, bool) {
	f, ok := wf.pendingFutures[scheduleEventID]
	return f, ok
}

func (wf *WfState) RemoveFuture(scheduleEventID int) {
	delete(wf.pendingFutures, scheduleEventID)
}

func (wf *WfState) Commands() []*command.Command {
	return wf.commands
}

func (wf *WfState) AddCommand(cmd *command.Command) {
	wf.commands = append(wf.commands, cmd)
}

func (wf *WfState) RemoveCommandByEventID(eventID int) *command.Command {
	for i, c := range wf.commands {
		if c.ID == eventID {
			wf.commands = append(wf.commands[:i], wf.commands[i+1:]...)
			return c
		}
	}

	return nil
}

func (wf *WfState) RemoveCommand(cmd command.Command) {
	for i, c := range wf.commands {
		if *c == cmd {
			// TODO: Move to state machines?
			c.State = command.CommandState_Done

			wf.commands = append(wf.commands[:i], wf.commands[i+1:]...)
			return
		}
	}
}

func (wf *WfState) ClearCommands() {
	wf.commands = []*command.Command{}
}

func (wf *WfState) CreateSignalChannel(name string) sync.Channel {
	cs := sync.NewBufferedChannel(10_000)
	wf.signalChannels[name] = cs
	return cs
}

func (wf *WfState) GetSignalChannel(name string) sync.Channel {
	cs, ok := wf.signalChannels[name]
	if ok {
		return cs
	}

	return wf.CreateSignalChannel(name)
}

func (wf *WfState) SetReplaying(replaying bool) {
	wf.replaying = replaying
}

func (wf *WfState) Replaying() bool {
	return wf.replaying
}

func (wf *WfState) SetTime(t time.Time) {
	wf.time = t
}

func (wf *WfState) Time() time.Time {
	return wf.time
}

func (wf *WfState) Instance() core.WorkflowInstance {
	return wf.instance
}
