package workflow

import (
	"fmt"
	"time"

	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/contextvalue"
	"github.com/cschleiden/go-workflows/internal/log"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/cschleiden/go-workflows/internal/workflowtracer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ScheduleTimer schedules a timer to fire after the given delay.
func ScheduleTimer(ctx Context, delay time.Duration) Future[any] {
	f := sync.NewFuture[any]()

	// If the context is already canceled, return immediately.
	if ctx.Err() != nil {
		f.Set(struct{}{}, ctx.Err())
		return f
	}

	wfState := workflowstate.WorkflowState(ctx)

	scheduleEventID := wfState.GetNextScheduleEventID()
	at := Now(ctx).Add(delay)

	timerCmd := command.NewScheduleTimerCommand(scheduleEventID, at)
	wfState.AddCommand(timerCmd)
	wfState.TrackFuture(scheduleEventID, workflowstate.AsDecodingSettable(contextvalue.Converter(ctx), fmt.Sprintf("timer:%v", delay), f))

	cancelReceiver := &sync.Receiver[struct{}]{
		Receive: func(v struct{}, ok bool) {
			timerCmd.Cancel()

			// Remove the timer future from the workflow state and mark it as canceled if it hasn't already fired. This is different
			// from subworkflow behavior, where we want to wait for the subworkflow to complete before proceeding. Here we can
			// continue right away.
			if fi, ok := f.(sync.FutureInternal[any]); ok {
				if !fi.Ready() {
					wfState.RemoveFuture(scheduleEventID)
					f.Set(v, Canceled)
				}
			}
		},
	}

	ctx, span := workflowtracer.Tracer(ctx).Start(ctx, "ScheduleTimer",
		trace.WithAttributes(
			attribute.Int64(log.DurationKey, int64(delay/time.Millisecond)),
			attribute.String(log.NowKey, Now(ctx).String()),
			attribute.String(log.AtKey, at.String()),
		))
	defer span.End()

	// Check if the context is cancelable
	if c, cancelable := ctx.Done().(sync.CancelChannel); cancelable {
		// Register a callback for when it's canceled. The only operation on the `Done` channel
		// is that it's closed when the context is canceled.
		c.AddReceiveCallback(cancelReceiver)

		timerCmd.WhenDone(func() {
			c.RemoveReceiveCallback(cancelReceiver)
		})
	}

	return f
}
