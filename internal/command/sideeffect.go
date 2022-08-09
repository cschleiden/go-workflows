package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
)

type SideEffectCommand struct {
	command

	result payload.Payload
}

var _ Command = (*SideEffectCommand)(nil)

func NewSideEffectCommand(id int64) *SideEffectCommand {
	return &SideEffectCommand{
		command: command{
			state: CommandState_Pending,
			id:    id,
		},
	}
}

func (c *SideEffectCommand) SetResult(result payload.Payload) {
	c.result = result
}

func (c *SideEffectCommand) Type() string {
	return "SideEffect"
}

func (c *SideEffectCommand) Commit(clock clock.Clock) *CommandResult {
	c.commit()

	return &CommandResult{
		Events: []history.Event{
			history.NewPendingEvent(
				clock.Now(),
				history.EventType_SideEffectResult,
				&history.SideEffectResultAttributes{
					Result: c.result,
				},
				history.ScheduleEventID(c.id),
			),
		},
	}
}
