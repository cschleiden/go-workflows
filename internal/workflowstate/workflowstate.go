package workflowstate

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/log"
)

type key int

var workflowCtxKey key

type DecodingSettable func(v payload.Payload, err error) error

// Use this to track futures for the workflow state. It's required to map the generic Future interface
// to a type without type parameters.
func AsDecodingSettable[T any](cv converter.Converter, f sync.SettableFuture[T]) DecodingSettable {
	return func(v payload.Payload, err error) error {
		if f.HasValue() {
			return fmt.Errorf("future already has value")
		}

		var t T
		if v != nil {
			if err := cv.From(v, &t); err != nil {
				return fmt.Errorf("failed to decode future: %v", err)
			}
		}

		f.Set(t, err)

		return nil
	}
}

type signalChannel struct {
	receive func(payload.Payload)
	channel interface{}
}

type WfState struct {
	instance        *core.WorkflowInstance
	scheduleEventID int64
	commands        []command.Command
	pendingFutures  map[int64]DecodingSettable
	replaying       bool

	pendingSignals map[string][]payload.Payload
	signalChannels map[string]*signalChannel

	logger *slog.Logger

	clock clock.Clock
	time  time.Time
}

func NewWorkflowState(instance *core.WorkflowInstance, logger *slog.Logger, clock clock.Clock) *WfState {
	state := &WfState{
		instance:        instance,
		commands:        []command.Command{},
		scheduleEventID: 1,
		pendingFutures:  map[int64]DecodingSettable{},

		pendingSignals: map[string][]payload.Payload{},
		signalChannels: make(map[string]*signalChannel),

		clock: clock,
	}

	state.logger = NewReplayLogger(state, logger.With(
		log.InstanceIDKey, instance.InstanceID,
		log.ExecutionIDKey, instance.ExecutionID))

	return state
}

func WorkflowState(ctx sync.Context) *WfState {
	return ctx.Value(workflowCtxKey).(*WfState)
}

func WithWorkflowState(ctx sync.Context, wfState *WfState) sync.Context {
	return sync.WithValue(ctx, workflowCtxKey, wfState)
}

func (wf *WfState) GetNextScheduleEventID() int64 {
	scheduleEventID := wf.scheduleEventID
	wf.scheduleEventID++
	return scheduleEventID
}

func (wf *WfState) TrackFuture(scheduleEventID int64, f DecodingSettable) {
	wf.pendingFutures[scheduleEventID] = f
}

func (wf *WfState) HasPendingFutures() bool {
	return len(wf.pendingFutures) > 0
}

func (wf *WfState) FutureByScheduleEventID(scheduleEventID int64) (DecodingSettable, bool) {
	f, ok := wf.pendingFutures[scheduleEventID]
	return f, ok
}

func (wf *WfState) RemoveFuture(scheduleEventID int64) {
	delete(wf.pendingFutures, scheduleEventID)
}

func (wf *WfState) Commands() []command.Command {
	return wf.commands
}

func (wf *WfState) AddCommand(cmd command.Command) {
	wf.commands = append(wf.commands, cmd)
}

func (wf *WfState) CommandByScheduleEventID(scheduleEventID int64) command.Command {
	for _, c := range wf.commands {
		if c.ID() == scheduleEventID {
			return c
		}
	}

	return nil
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

func (wf *WfState) Instance() *core.WorkflowInstance {
	return wf.instance
}

func (wf *WfState) Logger() *slog.Logger {
	return wf.logger
}
