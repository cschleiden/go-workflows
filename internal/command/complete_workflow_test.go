package command

import (
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCompleteWorkflowCommand_StateTransitions(t *testing.T) {
	tests := []struct {
		name string
		f    func(t *testing.T, c *CompleteWorkflowCommand, clock clock.Clock)
	}{
		{"Execute records finished event", func(t *testing.T, c *CompleteWorkflowCommand, clock clock.Clock) {
			assertExecuteWithEvent(t, c, CommandState_Done, history.EventType_WorkflowExecutionFinished)
		}},
		{"Commit", func(t *testing.T, c *CompleteWorkflowCommand, _ clock.Clock) {
			require.Equal(t, CommandState_Pending, c.State())

			c.Commit()
			require.Equal(t, CommandState_Done, c.State())

			assertExecuteNoEvent(t, c, CommandState_Done)
		}},

		{"Done_after_commit", func(t *testing.T, c *CompleteWorkflowCommand, clock clock.Clock) {
			c.Commit()

			require.PanicsWithError(t, "invalid state transition for command CompleteWorkflow: Done -> Done", func() {
				c.Done()
			})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clock.NewMock()
			cmd := NewCompleteWorkflowCommand(1, core.NewWorkflowInstance(uuid.NewString(), ""), payload.Payload{}, nil)

			tt.f(t, cmd, clock)
		})
	}
}
