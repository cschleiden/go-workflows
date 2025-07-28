package command

import (
	"github.com/benbjohnson/clock"
	"github.com/google/uuid"

	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/core"
)

type ScheduleSubWorkflowCommand struct {
	cancelableCommand

	Queue          core.Queue
	Instance       *core.WorkflowInstance
	Metadata       *metadata.WorkflowMetadata
	WorkflowSpanID [8]byte

	Name   string
	Inputs []payload.Payload
}

var _ CancelableCommand = (*ScheduleSubWorkflowCommand)(nil)

func NewScheduleSubWorkflowCommand(
	id int64, parentInstance *core.WorkflowInstance, subWorkflowQueue core.Queue, subWorkflowInstanceID,
	name string, inputs []payload.Payload, metadata *metadata.WorkflowMetadata, workflowSpanID [8]byte,
) *ScheduleSubWorkflowCommand {
	if subWorkflowInstanceID == "" {
		subWorkflowInstanceID = uuid.New().String()
	}

	return &ScheduleSubWorkflowCommand{
		cancelableCommand: cancelableCommand{
			command: command{
				id:    id,
				name:  "ScheduleSubWorkflow",
				state: CommandState_Pending,
			},
		},

		Queue:          subWorkflowQueue,
		Instance:       core.NewSubWorkflowInstance(subWorkflowInstanceID, uuid.NewString(), parentInstance, id),
		Metadata:       metadata,
		WorkflowSpanID: workflowSpanID,

		Name:   name,
		Inputs: inputs,
	}
}

func (c *ScheduleSubWorkflowCommand) Execute(clock clock.Clock) *CommandResult {
	switch c.state {
	case CommandState_Pending:
		c.state = CommandState_Committed
		return &CommandResult{
			// Record scheduled sub-workflow for source workflow instance
			Events: []*history.Event{
				history.NewPendingEvent(
					clock.Now(),
					history.EventType_SubWorkflowScheduled,
					&history.SubWorkflowScheduledAttributes{
						SubWorkflowInstance: c.Instance,
						Metadata:            c.Metadata,
						Name:                c.Name,
						Inputs:              c.Inputs,
					},
					history.ScheduleEventID(c.id),
				),
			},
			// Send event to new workflow instance
			WorkflowEvents: []*history.WorkflowEvent{
				{
					WorkflowInstance: c.Instance,
					HistoryEvent: history.NewPendingEvent(
						clock.Now(),
						history.EventType_WorkflowExecutionStarted,
						&history.ExecutionStartedAttributes{
							Queue:          c.Queue,
							Name:           c.Name,
							Inputs:         c.Inputs,
							Metadata:       c.Metadata,
							WorkflowSpanID: c.WorkflowSpanID,
						},
					),
				},
			},
		}

	case CommandState_CancelPending:
		c.state = CommandState_Canceled

		return &CommandResult{
			// Record that cancellation was requested
			Events: []*history.Event{
				history.NewPendingEvent(
					clock.Now(),
					history.EventType_SubWorkflowCancellationRequested,
					&history.SubWorkflowCancellationRequestedAttributes{
						SubWorkflowInstance: c.Instance,
					},
					history.ScheduleEventID(c.id),
				),
			},

			// Send cancellation event to sub-workflow
			WorkflowEvents: []*history.WorkflowEvent{
				{
					WorkflowInstance: c.Instance,
					HistoryEvent:     history.NewWorkflowCancellationEvent(clock.Now()),
				},
			},
		}
	}

	return nil
}
