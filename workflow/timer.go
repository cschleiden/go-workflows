package workflow

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

func ScheduleTimer(ctx sync.Context, delay time.Duration) sync.Future {
	wfState := workflowstate.WorkflowState(ctx)

	scheduleEventID := wfState.GetNextScheduleEventID()

	timerCmd := command.NewScheduleTimerCommand(scheduleEventID, Now(ctx).Add(delay))
	wfState.AddCommand(&timerCmd)

	t := sync.NewFuture()
	wfState.TrackFuture(scheduleEventID, t)

	if d := ctx.Done(); d != nil {
		if c, ok := d.(sync.ChannelInternal); ok {
			c.AddReceiveCallback(func(v interface{}) {
				wfState.RemoveFuture(scheduleEventID)
				t.Set(nil, sync.Canceled)
			})
		}
	}

	return t
}
