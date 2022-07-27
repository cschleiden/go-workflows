package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/history"
)

type CancelTimerCommand struct {
	command

	TimerScheduleEventID int64
}

var _ Command = (*CancelTimerCommand)(nil)

func NewCancelTimerCommand(id int64, timerScheduleEventID int64) *CancelTimerCommand {
	return &CancelTimerCommand{
		command: command{
			state: CommandState_Pending,
			id:    id,
		},
		TimerScheduleEventID: timerScheduleEventID,
	}
}

func (*CancelTimerCommand) Type() string {
	return "CancelTimer"
}

func (c *CancelTimerCommand) Commit(clock clock.Clock) *CommandResult {
	c.commit()

	return &CommandResult{
		Events: []history.Event{
			history.NewPendingEvent(
				clock.Now(),
				history.EventType_TimerCanceled,
				&history.TimerCanceledAttributes{},
				history.ScheduleEventID(c.TimerScheduleEventID),
			),
		},
	}
}
