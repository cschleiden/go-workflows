package workflows

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/workflow"
)

const (
	maxIterations = 10

	// maxBatchesPerIteration caps how many batch-removal activity calls run
	// per timer iteration. Each call removes up to BatchSize (default 100)
	// instances, so this allows draining up to maxBatchesPerIteration × 100
	// expired instances per cycle.
	maxBatchesPerIteration = 100

	UpdateExpirationSignal = "update-expiration"
)

func ExpireWorkflowInstances(ctx workflow.Context, delay time.Duration) error {
	logger := workflow.Logger(ctx)

	updates := workflow.NewSignalChannel[time.Duration](ctx, UpdateExpirationSignal)

	for i := 0; i < maxIterations; i++ {
		tctx, cancelTimer := workflow.WithCancel(ctx)
		t := workflow.ScheduleTimer(tctx, delay)

		timerFired := false
		for !timerFired {
			workflow.Select(ctx,
				workflow.Receive(updates, func(ctx workflow.Context, s time.Duration, _ bool) {
					delay = s

					cancelTimer()
					tctx, cancelTimer = workflow.WithCancel(ctx)
					t = workflow.ScheduleTimer(tctx, delay)
				}),
				workflow.Await(t, func(ctx sync.Context, _ workflow.Future[any]) {
					timerFired = true
				}),
			)
		}

		before := workflow.Now(ctx).Add(-delay)

		logger.Info("removing workflow instances", slog.Time("before", before))

		// Loop the activity to drain expired instances incrementally. Each
		// invocation removes up to one batch and gets its own activity lock
		// timeout window, avoiding the death-spiral where a single call tries
		// to process the entire backlog and always exceeds the deadline.
		for batch := 0; batch < maxBatchesPerIteration; batch++ {
			var a *Activities
			removed, err := workflow.ExecuteActivity[int](
				ctx, workflow.ActivityOptions{
					Queue: core.QueueSystem,
					RetryOptions: workflow.RetryOptions{
						MaxAttempts: 2,
					},
				}, a.RemoveWorkflowInstances, before).Get(ctx)
			if err != nil {
				if errors.As(err, &backend.ErrNotSupported{}) {
					logger.Warn("removing workflow instances not supported")
					return nil
				}

				logger.Error("removing workflow instances",
					slog.Any("error", err), slog.Int("batch", batch))
				break
			}

			logger.Info("removed workflow instances",
				slog.Int("removed", removed), slog.Int("batch", batch))

			if removed == 0 {
				break
			}

			// Yield between batches so task processing can acquire the DB lock.
			// Without this, continuous DELETE batches on large databases starve
			// GetWorkflowTask/GetActivityTask.
			if err := workflow.Sleep(ctx, 20*time.Second); err != nil {
				break
			}
		}
	}

	return workflow.ContinueAsNew(ctx, delay)
}

type Activities struct {
	Backend backend.Backend
}

func (a *Activities) RemoveWorkflowInstances(ctx context.Context, before time.Time) (int, error) {
	return a.Backend.RemoveWorkflowInstances(ctx, backend.RemoveFinishedBefore(before))
}
