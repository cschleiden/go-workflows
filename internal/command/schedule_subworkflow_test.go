package command

import (
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestScheduleSubWorkflowCommand_StateTransitions(t *testing.T) {
	tests := []struct {
		name string
		f    func(t *testing.T, c *ScheduleSubWorkflowCommand, clock clock.Clock)
	}{
		{"Execute schedules subworkflow", func(t *testing.T, c *ScheduleSubWorkflowCommand, clock clock.Clock) {
			r := assertExecuteWithEvent(t, c, CommandState_Committed, history.EventType_SubWorkflowScheduled)
			require.Equal(t, r.WorkflowEvents[0].HistoryEvent.Type, history.EventType_WorkflowExecutionStarted)
		}},
		{"Cancel after schedule yields cancel event", func(t *testing.T, c *ScheduleSubWorkflowCommand, clock clock.Clock) {
			assertExecuteWithEvent(t, c, CommandState_Committed, history.EventType_SubWorkflowScheduled)

			c.Cancel()
			require.Equal(t, CommandState_CancelPending, c.State())

			r := assertExecuteWithEvent(t, c, CommandState_Canceled, history.EventType_SubWorkflowCancellationRequested)
			require.Equal(t, r.WorkflowEvents[0].HistoryEvent.Type, history.EventType_WorkflowExecutionCanceled)
		}},
		{"Cancel after commit yields cancel event", func(t *testing.T, c *ScheduleSubWorkflowCommand, clock clock.Clock) {
			c.Commit()

			c.Cancel()
			require.Equal(t, CommandState_CancelPending, c.State())

			assertExecuteWithEvent(t, c, CommandState_Canceled, history.EventType_SubWorkflowCancellationRequested)
		}},
		{"Commit", func(t *testing.T, c *ScheduleSubWorkflowCommand, _ clock.Clock) {
			require.Equal(t, CommandState_Pending, c.State())

			c.Commit()
			require.Equal(t, CommandState_Committed, c.State())

			assertExecuteNoEvent(t, c, CommandState_Committed)
		}},
		{"Cancel_MultipleTimes", func(t *testing.T, c *ScheduleSubWorkflowCommand, clock clock.Clock) {
			c.Cancel()
			require.Equal(t, CommandState_Canceled, c.State())

			c.Cancel()
			require.Equal(t, CommandState_Canceled, c.State())
		}},
		{"HandleCancel", func(t *testing.T, c *ScheduleSubWorkflowCommand, clock clock.Clock) {
			c.Commit()

			c.Cancel()
			require.Equal(t, CommandState_CancelPending, c.State())

			c.HandleCancel()
			require.Equal(t, CommandState_Canceled, c.State())

			assertExecuteNoEvent(t, c, CommandState_Done)
		}},
		{"Done_after_commit", func(t *testing.T, c *ScheduleSubWorkflowCommand, clock clock.Clock) {
			c.Commit()

			c.Done()
			require.Equal(t, CommandState_Done, c.State())
		}},
		{"Done_after_cancel", func(t *testing.T, c *ScheduleSubWorkflowCommand, clock clock.Clock) {
			c.Cancel()

			c.Done()
			require.Equal(t, CommandState_Done, c.State())
		}},
		{"Invalid_HandleCancel", func(t *testing.T, c *ScheduleSubWorkflowCommand, clock clock.Clock) {
			c.Commit()

			require.PanicsWithError(t, "invalid state transition for command ScheduleSubWorkflow: Committed -> Canceled", func() {
				c.HandleCancel()
			})
		}},
		{"Invalid_Commit", func(t *testing.T, c *ScheduleSubWorkflowCommand, clock clock.Clock) {
			c.Cancel()

			require.PanicsWithError(t, "invalid state transition for command ScheduleSubWorkflow: Canceled -> Committed", func() {
				c.Commit()
			})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clock.NewMock()

			parentInstance := core.NewWorkflowInstance(uuid.NewString(), "")

			cmd := NewScheduleSubWorkflowCommand(1, parentInstance, uuid.NewString(), "SubWorkflow", []payload.Payload{}, &metadata.WorkflowMetadata{})

			tt.f(t, cmd, clock)
		})
	}
}
