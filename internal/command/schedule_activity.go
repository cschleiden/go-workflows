package command

import (
	"github.com/benbjohnson/clock"

	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/core"
)

type ScheduleActivityCommand struct {
	command

	Name     string
	Inputs   []payload.Payload
	Attempt  int
	Metadata *metadata.WorkflowMetadata
	Queue    core.Queue
}

var _ Command = (*ScheduleActivityCommand)(nil)

func NewScheduleActivityCommand(id int64, name string, inputs []payload.Payload, attempt int, metadata *metadata.WorkflowMetadata, queue core.Queue) *ScheduleActivityCommand {
	return &ScheduleActivityCommand{
		command: command{
			id:    id,
			name:  "ScheduleActivity",
			state: CommandState_Pending,
		},
		Name:     name,
		Attempt:  attempt,
		Inputs:   inputs,
		Metadata: metadata,
		Queue:    queue,
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
				Name:     c.Name,
				Inputs:   c.Inputs,
				Attempt:  c.Attempt,
				Metadata: c.Metadata,
				Queue:    c.Queue,
			},
			history.ScheduleEventID(c.id))

		return &CommandResult{
			Events:         []*history.Event{event},
			ActivityEvents: []*history.Event{event},
		}
	}

	return nil
}
