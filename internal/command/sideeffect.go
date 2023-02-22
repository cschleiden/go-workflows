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
			id:    id,
			name:  "SideEffect",
			state: CommandState_Pending,
		},
	}
}

func (c *SideEffectCommand) SetResult(result payload.Payload) {
	c.result = result
}

func (c *SideEffectCommand) Commit() {
	switch c.state {
	case CommandState_Pending:
		c.state = CommandState_Done

	default:
		c.invalidStateTransition(CommandState_Done)
	}
}

func (c *SideEffectCommand) Execute(clock clock.Clock) *CommandResult {
	switch c.state {
	case CommandState_Pending:
		// Side effects are only added to the history, transition to Done
		c.state = CommandState_Done

		return &CommandResult{
			Events: []*history.Event{
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

	return nil
}

func (c *SideEffectCommand) Done() {
	switch c.state {
	case CommandState_Pending, CommandState_Committed:
		c.state = CommandState_Done

	default:
		c.invalidStateTransition(CommandState_Done)
	}
}
