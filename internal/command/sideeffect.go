package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
)

type sideEffectCommand struct {
	command

	result payload.Payload
}

var _ Command = (*sideEffectCommand)(nil)

func NewSideEffectCommand(id int64, result payload.Payload) *sideEffectCommand {
	return &sideEffectCommand{
		command: command{
			state: CommandState_Pending,
			id:    id,
		},
		result: result,
	}
}

func (c *sideEffectCommand) Type() string {
	return "SideEffect"
}

func (c *sideEffectCommand) Commit(clock clock.Clock) *CommandResult {
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
