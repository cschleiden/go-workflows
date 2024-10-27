package command

import (
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/internal/tracing"
)

type ScheduleTimerCommand struct {
	cancelableCommand

	at           time.Time
	name         string
	traceContext tracing.Context
}

var _ CancelableCommand = (*ScheduleTimerCommand)(nil)

func NewScheduleTimerCommand(id int64, at time.Time, name string, traceContext tracing.Context) *ScheduleTimerCommand {
	return &ScheduleTimerCommand{
		cancelableCommand: cancelableCommand{
			command: command{
				id:    id,
				name:  "ScheduleTimer",
				state: CommandState_Pending,
			},
		},
		at:           at,
		name:         name,
		traceContext: traceContext,
	}
}

func (c *ScheduleTimerCommand) Execute(clock clock.Clock) *CommandResult {
	switch c.state {
	case CommandState_Pending:
		c.state = CommandState_Committed

		now := clock.Now()

		return &CommandResult{
			Events: []*history.Event{
				history.NewPendingEvent(
					now,
					history.EventType_TimerScheduled,
					&history.TimerScheduledAttributes{
						At:   c.at,
						Name: c.name,
					},
					history.ScheduleEventID(c.id),
				),
			},

			TimerEvents: []*history.Event{
				history.NewPendingEvent(
					clock.Now(),
					history.EventType_TimerFired,
					&history.TimerFiredAttributes{
						ScheduledAt:  now,
						At:           c.at,
						Name:         c.name,
						TraceContext: c.traceContext,
					},
					history.ScheduleEventID(c.id),
					history.VisibleAt(c.at),
				),
			},
		}

	case CommandState_CancelPending:
		c.state = CommandState_Canceled

		return &CommandResult{
			Events: []*history.Event{
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
