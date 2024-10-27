package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/core"
	"github.com/google/uuid"
)

type ContinueAsNewCommand struct {
	command

	Instance *core.WorkflowInstance
	Name     string
	Metadata *metadata.WorkflowMetadata
	Inputs   []payload.Payload
	Result   payload.Payload

	ContinuedExecutionID string
}

var _ Command = (*ContinueAsNewCommand)(nil)

func NewContinueAsNewCommand(id int64, instance *core.WorkflowInstance, result payload.Payload, name string, metadata *metadata.WorkflowMetadata, inputs []payload.Payload) *ContinueAsNewCommand {
	return &ContinueAsNewCommand{
		command: command{
			id:    id,
			name:  "ContinueAsNew",
			state: CommandState_Pending,
		},
		Instance:             instance,
		Name:                 name,
		Metadata:             metadata,
		Inputs:               inputs,
		Result:               result,
		ContinuedExecutionID: uuid.NewString(),
	}
}

func (c *ContinueAsNewCommand) Execute(clock clock.Clock) *CommandResult {
	switch c.state {
	case CommandState_Pending:
		var continuedInstance *core.WorkflowInstance
		if c.Instance.SubWorkflow() {
			// If the current workflow execution was a sub-workflow, ensure the new workflow execution is also a sub-workflow.
			// This will guarantee that the finished event for the new execution will be delivered to the right parent instance
			continuedInstance = core.NewSubWorkflowInstance(
				c.Instance.InstanceID, c.ContinuedExecutionID, c.Instance.Parent, c.Instance.ParentEventID)
		} else {
			continuedInstance = core.NewWorkflowInstance(c.Instance.InstanceID, c.ContinuedExecutionID)
		}

		c.state = CommandState_Committed
		return &CommandResult{
			State: core.WorkflowInstanceStateContinuedAsNew,
			Events: []*history.Event{
				// End the current workflow execution
				history.NewPendingEvent(
					clock.Now(),
					history.EventType_WorkflowExecutionContinuedAsNew,
					&history.ExecutionContinuedAsNewAttributes{
						Result:               c.Result,
						ContinuedExecutionID: c.ContinuedExecutionID,
					},
				),
			},
			WorkflowEvents: []*history.WorkflowEvent{
				// Schedule a new workflow execution
				{
					WorkflowInstance: continuedInstance,
					HistoryEvent: history.NewPendingEvent(
						clock.Now(),
						history.EventType_WorkflowExecutionStarted,
						&history.ExecutionStartedAttributes{
							Name:     c.Name,
							Metadata: c.Metadata,
							Inputs:   c.Inputs,
						},
					),
				},
			},
		}
	}

	return nil
}
