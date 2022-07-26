package command

import (
	"time"

	"github.com/benbjohnson/clock"
	"github.com/ticctech/go-workflows/internal/history"
)

type ScheduleTimerCommand struct {
	command

	at time.Time
}

var _ Command = (*ScheduleTimerCommand)(nil)

func NewScheduleTimerCommand(id int64, at time.Time) *ScheduleTimerCommand {
	return &ScheduleTimerCommand{
		command: command{
			state: CommandState_Pending,
			id:    id,
		},
		at: at,
	}
}

func (c *ScheduleTimerCommand) Type() string {
	return "ScheduleTimer"
}

func (c *ScheduleTimerCommand) Commit(clock clock.Clock) *CommandResult {
	c.commit()

	return &CommandResult{
		Events: []history.Event{
			history.NewPendingEvent(
				clock.Now(),
				history.EventType_TimerScheduled,
				&history.TimerScheduledAttributes{
					At: c.at,
				},
				history.ScheduleEventID(c.id),
			),
		},

		TimerEvents: []history.Event{
			history.NewPendingEvent(
				clock.Now(),
				history.EventType_TimerFired,
				&history.TimerFiredAttributes{
					At: c.at,
				},
				history.ScheduleEventID(c.id),
				history.VisibleAt(c.at),
			),
		},
	}
}
