package workflow

import (
	"time"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/sync"
)

func ScheduleTimer(ctx sync.Context, delay time.Duration) (sync.Future, error) {
	wfState := getWfState(ctx)

	eventID := wfState.eventID
	wfState.eventID++

	timerCommand := command.NewScheduleTimerCommand(eventID, time.Now().UTC().Add(delay))
	wfState.addCommand(timerCommand)

	t := sync.NewFuture()
	wfState.pendingFutures[eventID] = t

	if d := ctx.Done(); d != nil {
		if c, ok := d.(sync.ChannelInternal); ok {
			c.AddReceiveCallback(func(v interface{}) {
				delete(wfState.pendingFutures, eventID)
				t.Set(nil, sync.Canceled)
			})
		}
	}

	return t, nil
}
