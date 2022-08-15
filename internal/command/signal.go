package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
)

type SignalWorkflowCommand struct {
	command

	Instance *core.WorkflowInstance

	Name string
	Arg  payload.Payload
}

var _ Command = (*SignalWorkflowCommand)(nil)

func NewSignalWorkflowCommand(
	id int64, workflowInstanceID, name string, arg payload.Payload,
) *SignalWorkflowCommand {
	return &SignalWorkflowCommand{
		command: command{
			id:    id,
			name:  "SignalWorkflow",
			state: CommandState_Pending,
		},

		Instance: core.NewWorkflowInstance(workflowInstanceID, ""), // TODO: Do we need a special identifier for an empty execution id?

		Name: name,
		Arg:  arg,
	}
}

func (c *SignalWorkflowCommand) Execute(clock clock.Clock) *CommandResult {
	switch c.state {
	case CommandState_Pending:
		c.state = CommandState_Done

		return &CommandResult{
			// Record signal requested
			Events: []history.Event{
				history.NewPendingEvent(
					clock.Now(),
					history.EventType_SignalWorkflow,
					&history.SignalWorkflowAttributes{
						Name: c.Name,
						Arg:  c.Arg,
					},
					history.ScheduleEventID(c.id),
				),
			},
			// Send event to workflow instance
			WorkflowEvents: []history.WorkflowEvent{
				{
					WorkflowInstance: c.Instance,
					HistoryEvent: history.NewPendingEvent(
						clock.Now(),
						history.EventType_SignalReceived,
						&history.SignalReceivedAttributes{
							Name: c.Name,
							Arg:  c.Arg,
						},
					),
				},
			},
		}
	}

	return nil
}

func (c *SignalWorkflowCommand) Done() {
	switch c.state {
	case CommandState_Pending, CommandState_Committed:
		c.state = CommandState_Done
	default:
		c.invalidStateTransition(CommandState_Done)
	}
}
