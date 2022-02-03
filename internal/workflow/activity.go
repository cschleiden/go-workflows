package workflow

import (
	"log"
	"math"
	"time"

	a "github.com/cschleiden/go-dt/internal/args"
	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/fn"
	"github.com/cschleiden/go-dt/internal/payload"
	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/pkg/errors"
)

type Activity interface{}

type ActivityOptions struct {
	RetryOptions RetryOptions
}

var DefaultActivityOptions = ActivityOptions{
	RetryOptions: DefaultRetryOptions,
}

// ExecuteActivity schedules the given activity to be executed
func ExecuteActivity(ctx sync.Context, options ActivityOptions, activity Activity, args ...interface{}) sync.Future {
	r := sync.NewFuture()

	sync.Go(ctx, func(ctx sync.Context) {
		firstAttempt := Now(ctx)

		var result payload.Payload
		var err error

		var retryExpiration time.Time
		if options.RetryOptions.RetryTimeout != 0 {
			retryExpiration = firstAttempt.Add(options.RetryOptions.RetryTimeout)
		}

		for attempt := 0; attempt < options.RetryOptions.MaxAttempts; attempt++ {
			if !retryExpiration.IsZero() && Now(ctx).After(retryExpiration) {
				// Reached maximum retry time, abort retries
				break
			}

			f := executeActivity(ctx, options, activity, args...)

			err = f.Get(ctx, &result)
			if err != nil {
				backoffDuration := time.Duration(float64(options.RetryOptions.FirstRetryInterval) * math.Pow(options.RetryOptions.BackoffCoefficient, float64(attempt)))
				if options.RetryOptions.MaxRetryInterval > 0 {
					backoffDuration = time.Duration(math.Min(float64(backoffDuration), float64(options.RetryOptions.MaxRetryInterval)))
				}

				Sleep(ctx, backoffDuration)

				continue
			}

			break
		}

		r.Set(result, err)
	})

	return r
}

func executeActivity(ctx sync.Context, options ActivityOptions, activity Activity, args ...interface{}) sync.Future {
	f := sync.NewFuture()

	inputs, err := a.ArgsToInputs(converter.DefaultConverter, args...)
	if err != nil {
		f.Set(nil, errors.Wrap(err, "failed to convert activity input"))
		return f
	}

	wfState := getWfState(ctx)
	eventID := wfState.eventID
	wfState.eventID++

	name := fn.Name(activity)
	cmd := command.NewScheduleActivityTaskCommand(eventID, name, inputs)
	wfState.addCommand(&cmd)

	wfState.pendingFutures[eventID] = f

	// Handle cancellation
	if d := ctx.Done(); d != nil {
		if c, ok := d.(sync.ChannelInternal); ok {
			c.ReceiveNonBlocking(ctx, func(_ interface{}) {
				// Workflow has been canceled, check if the activity has already been scheduled
				if cmd.State == command.CommandState_Committed {
					// Command has already been committed, that means the activity has already been scheduled. Wait
					// until the activity is done.
					return
				}

				wfState.removeCommand(cmd)
				delete(wfState.pendingFutures, eventID)
				f.Set(nil, sync.Canceled)
			})
		}
	}

	return f
}

func trace(ctx sync.Context, args ...interface{}) {
	if !Replaying(ctx) {
		log.Println(args...)
	}
}
