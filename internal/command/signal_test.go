package command

import (
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/stretchr/testify/require"
)

func TestSignalWorkflowCommand_StateTransitions(t *testing.T) {
	tests := []struct {
		name string
		f    func(t *testing.T, c *SignalWorkflowCommand, clock clock.Clock)
	}{
		{"Execute records signal", func(t *testing.T, c *SignalWorkflowCommand, clock clock.Clock) {
			r := assertExecuteWithEvent(t, c, CommandState_Done, history.EventType_SignalWorkflow)
			require.Equal(t, r.WorkflowEvents[0].HistoryEvent.Type, history.EventType_SignalReceived)
		}},
		{"Commit", func(t *testing.T, c *SignalWorkflowCommand, _ clock.Clock) {
			require.Equal(t, CommandState_Pending, c.State())

			c.Done()
			require.Equal(t, CommandState_Done, c.State())

			assertExecuteNoEvent(t, c, CommandState_Done)
		}},

		{"Done_after_commit", func(t *testing.T, c *SignalWorkflowCommand, clock clock.Clock) {
			c.Commit()

			c.Done()
			require.Equal(t, CommandState_Done, c.State())
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clock.NewMock()
			cmd := NewSignalWorkflowCommand(1, "instance_id", "signal_name", payload.Payload{})
			tt.f(t, cmd, clock)
		})
	}
}
