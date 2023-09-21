package command

import (
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/stretchr/testify/require"
)

func TestScheduleActivityCommand_StateTransitions(t *testing.T) {
	tests := []struct {
		name string
		f    func(t *testing.T, c *ScheduleActivityCommand, clock clock.Clock)
	}{
		{"Execute schedules activity", func(t *testing.T, c *ScheduleActivityCommand, clock clock.Clock) {
			assertExecuteWithEvent(t, c, CommandState_Committed, history.EventType_ActivityScheduled)
		}},
		{"Commit", func(t *testing.T, c *ScheduleActivityCommand, _ clock.Clock) {
			require.Equal(t, CommandState_Pending, c.State())

			c.Commit()
			require.Equal(t, CommandState_Committed, c.State())

			assertExecuteNoEvent(t, c, CommandState_Committed)
		}},

		{"Done_after_commit", func(t *testing.T, c *ScheduleActivityCommand, clock clock.Clock) {
			c.Commit()

			c.Done()
			require.Equal(t, CommandState_Done, c.State())
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clock.NewMock()
			cmd := NewScheduleActivityCommand(1, "activity", []payload.Payload{}, &metadata.WorkflowMetadata{})

			tt.f(t, cmd, clock)
		})
	}
}
