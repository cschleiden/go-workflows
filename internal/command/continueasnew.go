package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/google/uuid"
)

type ContinueAsNewCommand struct {
	command

	Instance *core.WorkflowInstance
	Name     string
	Metadata *core.WorkflowMetadata
	Inputs   []payload.Payload
	Result   payload.Payload
}

var _ Command = (*ContinueAsNewCommand)(nil)

func NewContinueAsNewCommand(id int64, instance *core.WorkflowInstance, result payload.Payload, name string, metadata *core.WorkflowMetadata, inputs []payload.Payload) *ContinueAsNewCommand {
	return &ContinueAsNewCommand{
		command: command{
			id:    id,
			name:  "ContinueAsNew",
			state: CommandState_Pending,
		},
		Instance: instance,
		Name:     name,
		Metadata: metadata,
		Inputs:   inputs,
		Result:   result,
	}
}

func (c *ContinueAsNewCommand) Execute(clock clock.Clock) *CommandResult {
	switch c.state {
	case CommandState_Pending:
		continuedExecutionID := uuid.NewString()

		var continuedInstance *core.WorkflowInstance
		if c.Instance.SubWorkflow() {
			// If the current workflow execution was a sub-workflow, ensure the new workflow execution is also a sub-workflow.
			// This will guarantee that finished event for the new execution will be delivered to the right parent instance
			continuedInstance = core.NewSubWorkflowInstance(
				c.Instance.InstanceID, continuedExecutionID, c.Instance.Parent, c.Instance.ParentEventID)
		} else {
			continuedInstance = core.NewWorkflowInstance(c.Instance.InstanceID, continuedExecutionID)
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
						ContinuedExecutionID: continuedExecutionID,
					},
				),
			},
			WorkflowEvents: []history.WorkflowEvent{
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
