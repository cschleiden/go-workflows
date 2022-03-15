package workflow

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
)

func ScheduleTimer(ctx Context, delay time.Duration) Future[struct{}] {
	wfState := workflowstate.WorkflowState(ctx)

	scheduleEventID := wfState.GetNextScheduleEventID()

	timerCmd := command.NewScheduleTimerCommand(scheduleEventID, Now(ctx).Add(delay))
	wfState.AddCommand(&timerCmd)

	t := sync.NewFuture[struct{}]()
	wfState.TrackFuture(scheduleEventID, workflowstate.AsDecodingSettable(t))

	if d := ctx.Done(); d != nil {
		if c, ok := d.(sync.ChannelInternal[struct{}]); ok {
			c.AddReceiveCallback(func(v struct{}, ok bool) {
				wfState.RemoveFuture(scheduleEventID)
				t.Set(v, sync.Canceled)
			})
		}
	}

	return t
}
