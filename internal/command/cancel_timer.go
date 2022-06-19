package command

import (
	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/history"
)

type cancelTimerCommand struct {
	command

	TimerScheduleEventID int64
}

var _ Command = (*cancelTimerCommand)(nil)

func NewCancelTimerCommand(id int64, timerScheduleEventID int64) *cancelTimerCommand {
	return &cancelTimerCommand{
		command: command{
			state: CommandState_Pending,
			id:    id,
		},
		TimerScheduleEventID: timerScheduleEventID,
	}
}

func (*cancelTimerCommand) Type() string {
	return "CancelTimer"
}

func (c *cancelTimerCommand) Commit(clock clock.Clock) *CommandResult {
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
