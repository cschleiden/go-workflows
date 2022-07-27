package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/google/uuid"
)

type ScheduleSubWorkflowCommand struct {
	command

	Instance *core.WorkflowInstance
	Metadata *core.WorkflowMetadata

	Name   string
	Inputs []payload.Payload
}

var _ Command = (*ScheduleSubWorkflowCommand)(nil)

func NewScheduleSubWorkflowCommand(
	id int64, parentInstance *core.WorkflowInstance, subWorkflowInstanceID, name string, inputs []payload.Payload, metadata *core.WorkflowMetadata,
) *ScheduleSubWorkflowCommand {
	if subWorkflowInstanceID == "" {
		subWorkflowInstanceID = uuid.New().String()
	}

	return &ScheduleSubWorkflowCommand{
		command: command{
			state: CommandState_Pending,
			id:    id,
		},

		Instance: core.NewSubWorkflowInstance(subWorkflowInstanceID, uuid.NewString(), parentInstance.InstanceID, id),
		Metadata: metadata,

		Name:   name,
		Inputs: inputs,
	}
}

func (*ScheduleSubWorkflowCommand) Type() string {
	return "ScheduleSubWorkflow"
}

func (c *ScheduleSubWorkflowCommand) Commit(clock clock.Clock) *CommandResult {
	c.commit()

	return &CommandResult{
		// Record scheduled sub-workflow for source workflow instance
		Events: []history.Event{
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
		WorkflowEvents: []history.WorkflowEvent{
			{
				WorkflowInstance: c.Instance,
				HistoryEvent: history.NewPendingEvent(
					clock.Now(),
					history.EventType_WorkflowExecutionStarted,
					&history.ExecutionStartedAttributes{
						Name:     c.Name,
						Inputs:   c.Inputs,
						Metadata: c.Metadata,
					},
					history.ScheduleEventID(0),
				),
			},
		},
	}
}
