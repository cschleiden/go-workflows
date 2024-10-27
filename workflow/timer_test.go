package workflow

import (
	"log/slog"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/contextvalue"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowstate"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
)

func Test_Timer_Cancellation(t *testing.T) {
	state := workflowstate.NewWorkflowState(
		core.NewWorkflowInstance("a", ""), slog.Default(), noop.NewTracerProvider().Tracer("test"), clock.New())

	ctx, cancel := WithCancel(sync.Background())
	ctx = contextvalue.WithConverter(ctx, converter.DefaultConverter)
	ctx = workflowstate.WithWorkflowState(ctx, state)

	c := sync.NewCoroutine(ctx, func(ctx Context) error {
		f := ScheduleTimer(ctx, time.Second*1)
		f.Get(ctx)

		// Block workflow
		sync.NewFuture[int]().Get(ctx)

		return nil
	})
	c.Execute()
	require.False(t, c.Finished())

	// Fire timer
	cmd := state.CommandByScheduleEventID(1)
	cmd.Commit()
	cmd.Done()
	fs, ok := state.FutureByScheduleEventID(1)
	require.True(t, ok)
	fs.Set(nil, nil)

	c.Execute()
	require.False(t, c.Finished())

	cancel()
}
