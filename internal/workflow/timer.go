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

	command := command.NewScheduleTimerCommand(eventID, time.Now().UTC().Add(delay))
	wfState.addCommand(command)

	t := sync.NewFuture()
	wfState.pendingFutures[eventID] = t

	// TODO: Check if context is cancelable, and if listen to Done channel

	return t, nil
}
