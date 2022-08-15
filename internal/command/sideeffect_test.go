package command

import (
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/stretchr/testify/require"
)

func TestSideEffectCommand_StateTransitions(t *testing.T) {
	tests := []struct {
		name string
		f    func(t *testing.T, c *SideEffectCommand, clock clock.Clock)
	}{
		{"Execute records sideeffect result", func(t *testing.T, c *SideEffectCommand, clock clock.Clock) {
			assertExecuteWithEvent(t, c, CommandState_Done, history.EventType_SideEffectResult)
		}},
		{"Commit", func(t *testing.T, c *SideEffectCommand, _ clock.Clock) {
			require.Equal(t, CommandState_Pending, c.State())

			c.Commit()
			require.Equal(t, CommandState_Done, c.State())

			assertExecuteNoEvent(t, c, CommandState_Done)
		}},
		{"Done_after_commit", func(t *testing.T, c *SideEffectCommand, clock clock.Clock) {
			c.Commit()

			require.PanicsWithError(t, "invalid state transition for command SideEffect: Done -> Done", func() {
				c.Done()
			})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clock.NewMock()
			cmd := NewSideEffectCommand(1)

			tt.f(t, cmd, clock)
		})
	}
}
