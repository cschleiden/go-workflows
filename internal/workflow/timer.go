package workflow

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/sync"
)

func ScheduleTimer(ctx sync.Context, delay time.Duration) sync.Future {
	wfState := WorkflowState(ctx)

	scheduleEventID := wfState.GetNextScheduleEventID()

	timerCmd := command.NewScheduleTimerCommand(scheduleEventID, Now(ctx).Add(delay))
	wfState.AddCommand(&timerCmd)

	t := sync.NewFuture()
	wfState.pendingFutures[scheduleEventID] = t

	if d := ctx.Done(); d != nil {
		if c, ok := d.(sync.ChannelInternal); ok {
			c.AddReceiveCallback(func(v interface{}) {
				delete(wfState.pendingFutures, scheduleEventID)
				t.Set(nil, sync.Canceled)
			})
		}
	}

	return t
}
