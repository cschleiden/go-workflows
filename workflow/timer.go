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

	f := sync.NewFuture[struct{}]()
	wfState.TrackFuture(scheduleEventID, workflowstate.AsDecodingSettable(f))

	// Check if the channel is cancelable
	if c, cancelable := ctx.Done().(sync.CancelChannel); cancelable {
		// Is the context is canceled already?
		if _, ok := c.ReceiveNonBlocking(ctx); ok {
			// Remove the command, no need to schedule timer
			wfState.RemoveCommand(&timerCmd)

			// And remove the future from the state and mark it as canceled
			wfState.RemoveFuture(scheduleEventID)
			f.Set(struct{}{}, sync.Canceled)
		} else {
			// Otherwise register a callback for when it's canceled. The only operation on the `Done` channel
			// is that it's closed when the context is canceled.
			c.AddReceiveCallback(func(v struct{}, ok bool) {
				if timerCmd.State == command.CommandState_Committed {
					// If the timer command is already committed, create a cancel command to allow the backend
					// to clean up the scheduled timer message.
					cancelScheduleEventID := wfState.GetNextScheduleEventID()
					timerCancelationCmd := command.NewCancelTimerCommand(cancelScheduleEventID, scheduleEventID)
					wfState.AddCommand(&timerCancelationCmd)
				}

				// Remove the timer future from the workflow state and mark it as canceled if it hasn't already fired
				if fi, ok := f.(sync.FutureInternal[struct{}]); ok {
					if !fi.Ready() {
						wfState.RemoveFuture(scheduleEventID)
						f.Set(v, sync.Canceled)
					}
				}
			})
		}
	}

	return f
}
