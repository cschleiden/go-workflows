package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
)

type CancelSubWorkflowCommand struct {
	command

	SubWorkflowInstance *core.WorkflowInstance
}

var _ Command = (*CancelSubWorkflowCommand)(nil)

func NewCancelSubWorkflowCommand(id int64, subWorkflowInstance *core.WorkflowInstance) *CancelSubWorkflowCommand {
	return &CancelSubWorkflowCommand{
		command: command{
			state: CommandState_Pending,
			id:    id,
		},
		SubWorkflowInstance: subWorkflowInstance,
	}
}

func (*CancelSubWorkflowCommand) Type() string {
	return "CancelSubworkflow"
}

func (c *CancelSubWorkflowCommand) Commit(clock clock.Clock) *CommandResult {
	c.commit()

	return &CommandResult{
		// Record that cancellation was requested
		Events: []history.Event{
			history.NewPendingEvent(
				clock.Now(),
				history.EventType_SubWorkflowCancellationRequested,
				&history.SubWorkflowCancellationRequestedAttributes{
					SubWorkflowInstance: c.SubWorkflowInstance,
				},
				history.ScheduleEventID(c.id),
			),
		},

		// Send cancellation event to sub-workflow
		WorkflowEvents: []history.WorkflowEvent{
			{
				WorkflowInstance: c.SubWorkflowInstance,
				HistoryEvent:     history.NewWorkflowCancellationEvent(clock.Now()),
			},
		},
	}
}
