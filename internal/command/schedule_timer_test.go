package command

import (
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/stretchr/testify/require"
)

func TestScheduleTimerCommand_StateTransitions(t *testing.T) {
	tests := []struct {
		name string
		f    func(t *testing.T, c *ScheduleTimerCommand, clock clock.Clock)
	}{
		{"Execute schedules timer", func(t *testing.T, c *ScheduleTimerCommand, clock clock.Clock) {
			assertExecuteWithEvent(t, c, CommandState_Committed, history.EventType_TimerScheduled)
		}},
		{"Cancel after schedule yields cancel event", func(t *testing.T, c *ScheduleTimerCommand, clock clock.Clock) {
			assertExecuteWithEvent(t, c, CommandState_Committed, history.EventType_TimerScheduled)

			c.Cancel()
			require.Equal(t, CommandState_CancelPending, c.State())

			assertExecuteWithEvent(t, c, CommandState_Canceled, history.EventType_TimerCanceled)
		}},
		{"Cancel after commit yields cancel event", func(t *testing.T, c *ScheduleTimerCommand, clock clock.Clock) {
			c.Commit()

			c.Cancel()
			require.Equal(t, CommandState_CancelPending, c.State())

			assertExecuteWithEvent(t, c, CommandState_Canceled, history.EventType_TimerCanceled)
		}},
		{"Commit", func(t *testing.T, c *ScheduleTimerCommand, _ clock.Clock) {
			require.Equal(t, CommandState_Pending, c.State())

			c.Commit()
			require.Equal(t, CommandState_Committed, c.State())

			assertExecuteNoEvent(t, c, CommandState_Committed)
		}},
		{"Cancel_MultipleTimes", func(t *testing.T, c *ScheduleTimerCommand, clock clock.Clock) {
			c.Cancel()
			require.Equal(t, CommandState_Canceled, c.State())

			c.Cancel()
			require.Equal(t, CommandState_Canceled, c.State())
		}},
		{"HandleCancel", func(t *testing.T, c *ScheduleTimerCommand, clock clock.Clock) {
			c.Commit()

			c.Cancel()
			require.Equal(t, CommandState_CancelPending, c.State())

			c.HandleCancel()
			require.Equal(t, CommandState_Canceled, c.State())

			assertExecuteNoEvent(t, c, CommandState_Canceled)
		}},
		{"Done_after_commit", func(t *testing.T, c *ScheduleTimerCommand, clock clock.Clock) {
			c.Commit()

			c.Done()
			require.Equal(t, CommandState_Done, c.State())
		}},
		{"Done_after_cancel", func(t *testing.T, c *ScheduleTimerCommand, clock clock.Clock) {
			c.Cancel()

			c.Done()
			require.Equal(t, CommandState_Done, c.State())
		}},
		{"Invalid_HandleCancel", func(t *testing.T, c *ScheduleTimerCommand, clock clock.Clock) {
			c.Commit()

			require.PanicsWithError(t, "invalid state transition for command ScheduleTimer: Committed -> Canceled", func() {
				c.HandleCancel()
			})
		}},
		{"Invalid_Commit", func(t *testing.T, c *ScheduleTimerCommand, clock clock.Clock) {
			c.Cancel()

			require.PanicsWithError(t, "invalid state transition for command ScheduleTimer: Canceled -> Committed", func() {
				c.Commit()
			})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clock.NewMock()
			cmd := NewScheduleTimerCommand(1, clock.Now().Add(time.Second), "", nil)

			tt.f(t, cmd, clock)
		})
	}
}
