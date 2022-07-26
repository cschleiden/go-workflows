package command

import (
	"github.com/benbjohnson/clock"
	"github.com/ticctech/go-workflows/internal/history"
	"github.com/ticctech/go-workflows/internal/payload"
)

type ScheduleActivityCommand struct {
	command

	Name   string
	Inputs []payload.Payload
}

var _ Command = (*ScheduleActivityCommand)(nil)

func NewScheduleActivityCommand(id int64, name string, inputs []payload.Payload) Command {
	return &ScheduleActivityCommand{
		command: command{
			state: CommandState_Pending,
			id:    id,
		},
		Name:   name,
		Inputs: inputs,
	}
}

func (*ScheduleActivityCommand) Type() string {
	return "ScheduleActivity"
}

func (c *ScheduleActivityCommand) Commit(clock clock.Clock) *CommandResult {
	c.commit()

	event := history.NewPendingEvent(
		clock.Now(),
		history.EventType_ActivityScheduled,
		&history.ActivityScheduledAttributes{
			Name:   c.Name,
			Inputs: c.Inputs,
		},
		history.ScheduleEventID(c.id))

	return &CommandResult{
		Events:         []history.Event{event},
		ActivityEvents: []history.Event{event},
	}
}
