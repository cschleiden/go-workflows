package command

import (
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/history"
)

type ScheduleTimerCommand struct {
	cancelableCommand

	at time.Time
}

var _ CancelableCommand = (*ScheduleTimerCommand)(nil)

func NewScheduleTimerCommand(id int64, at time.Time) *ScheduleTimerCommand {
	return &ScheduleTimerCommand{
		cancelableCommand: cancelableCommand{
			command: command{
				id:    id,
				name:  "ScheduleTimer",
				state: CommandState_Pending,
			},
		},
		at: at,
	}
}

func (c *ScheduleTimerCommand) Execute(clock clock.Clock) *CommandResult {
	switch c.state {
	case CommandState_Pending:
		c.state = CommandState_Committed

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

	case CommandState_CancelPending:
		c.state = CommandState_Canceled

		return &CommandResult{
			Events: []history.Event{
				history.NewPendingEvent(
					clock.Now(),
					history.EventType_TimerCanceled,
					&history.TimerCanceledAttributes{},
					history.ScheduleEventID(c.id),
				),
			},
		}
	}

	return nil
}
