package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
)

type CompleteWorkflowCommand struct {
	command

	Instance *core.WorkflowInstance
	Result   payload.Payload
	Error    *workflowerrors.Error
}

var _ Command = (*CompleteWorkflowCommand)(nil)

func NewCompleteWorkflowCommand(id int64, instance *core.WorkflowInstance, result payload.Payload, err *workflowerrors.Error) *CompleteWorkflowCommand {
	return &CompleteWorkflowCommand{
		command: command{
			id:    id,
			name:  "CompleteWorkflow",
			state: CommandState_Pending,
		},
		Instance: instance,
		Result:   result,
		Error:    err,
	}
}

func (c *CompleteWorkflowCommand) Commit() {
	switch c.state {
	case CommandState_Pending:
		c.state = CommandState_Done

	default:
		c.invalidStateTransition(CommandState_Done)
	}
}

func (c *CompleteWorkflowCommand) Execute(clock clock.Clock) *CommandResult {
	switch c.state {
	case CommandState_Pending:
		c.state = CommandState_Done

		r := &CommandResult{
			State: core.WorkflowInstanceStateFinished,
			Events: []*history.Event{
				history.NewPendingEvent(
					clock.Now(),
					history.EventType_WorkflowExecutionFinished,
					&history.ExecutionCompletedAttributes{
						Result: c.Result,
						Error:  c.Error,
					},
					history.ScheduleEventID(0),
				),
			},
		}

		if c.Instance.SubWorkflow() {
			// Send completion message back to parent workflow instance
			var historyEvent *history.Event

			if c.Error != nil {
				// Sub workflow failed
				historyEvent = history.NewPendingEvent(
					clock.Now(),
					history.EventType_SubWorkflowFailed,
					&history.SubWorkflowFailedAttributes{
						Error: c.Error,
					},
					// Ensure the message gets sent back to the parent workflow with the right schedule event ID
					history.ScheduleEventID(c.Instance.ParentEventID),
				)
			} else {
				historyEvent = history.NewPendingEvent(
					clock.Now(),
					history.EventType_SubWorkflowCompleted,
					&history.SubWorkflowCompletedAttributes{
						Result: c.Result,
					},
					// Ensure the message gets sent back to the parent workflow with the right schedule event ID
					history.ScheduleEventID(c.Instance.ParentEventID),
				)
			}

			r.WorkflowEvents = []history.WorkflowEvent{
				{
					WorkflowInstance: c.Instance.Parent,
					HistoryEvent:     historyEvent,
				},
			}
		}

		return r
	}

	return nil
}
