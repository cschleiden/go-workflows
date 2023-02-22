package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
)

type ScheduleActivityCommand struct {
	command

	Name   string
	Inputs []payload.Payload
}

var _ Command = (*ScheduleActivityCommand)(nil)

func NewScheduleActivityCommand(id int64, name string, inputs []payload.Payload) *ScheduleActivityCommand {
	return &ScheduleActivityCommand{
		command: command{
			id:    id,
			name:  "ScheduleActivity",
			state: CommandState_Pending,
		},
		Name:   name,
		Inputs: inputs,
	}
}

func (c *ScheduleActivityCommand) Execute(clock clock.Clock) *CommandResult {
	switch c.state {
	case CommandState_Pending:
		c.state = CommandState_Committed

		event := history.NewPendingEvent(
			clock.Now(),
			history.EventType_ActivityScheduled,
			&history.ActivityScheduledAttributes{
				Name:   c.Name,
				Inputs: c.Inputs,
			},
			history.ScheduleEventID(c.id))

		return &CommandResult{
			Events:         []*history.Event{event},
			ActivityEvents: []*history.Event{event},
		}
	}

	return nil
}
