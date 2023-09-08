package tester

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

func Test_Timer(t *testing.T) {
	tester := NewWorkflowTester[timerResult](workflowTimer)
	start := tester.Now()

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	wr, _ := tester.WorkflowResult()
	require.True(t, start.Equal(wr.T1))

	e := start.Add(30 * time.Second)
	require.True(t, e.Equal(wr.T2), "expected %v, got %v", e, wr.T2)
}

type timerResult struct {
	T1 time.Time
	T2 time.Time
}

func workflowTimer(ctx workflow.Context) (timerResult, error) {
	t1 := workflow.Now(ctx)

	workflow.ScheduleTimer(ctx, 30*time.Second).Get(ctx)

	t2 := workflow.Now(ctx)

	workflow.ScheduleTimer(ctx, 30*time.Second).Get(ctx)

	return timerResult{
		T1: t1,
		T2: t2,
	}, nil
}

func Test_TimerHanging(t *testing.T) {
	wf := func(ctx workflow.Context) error {
		// Schedule timer and don't wait for it
		workflow.ScheduleTimer(ctx, 30*time.Second)

		return nil
	}

	tester := NewWorkflowTester[timerResult](wf)

	require.Panics(t, func() {
		tester.Execute(context.Background())
	})
}

func Test_TimerCancellation(t *testing.T) {
	tester := NewWorkflowTester[time.Time](workflowTimerCancellation)
	start := tester.Now()

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())

	wfR, _ := tester.WorkflowResult()
	require.True(t, start.Equal(wfR), "expected %v, got %v", start, wfR)
}

func workflowTimerCancellation(ctx workflow.Context) (time.Time, error) {
	tctx, cancel := workflow.WithCancel(ctx)
	t := workflow.ScheduleTimer(tctx, 30*time.Second)
	cancel()

	_, _ = t.Get(ctx)

	return workflow.Now(ctx), nil
}

func Test_TimerSubworkflowCancellation(t *testing.T) {
	tester := NewWorkflowTester[time.Time](workflowSubWorkflowTimerCancellation)
	tester.Registry().RegisterWorkflow(timerCancellationSubWorkflow)

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())

	_, wfErr := tester.WorkflowResult()
	require.Empty(t, wfErr)
}

func workflowSubWorkflowTimerCancellation(ctx workflow.Context) error {
	_, err := workflow.CreateSubWorkflowInstance[any](ctx, workflow.DefaultSubWorkflowOptions, timerCancellationSubWorkflow).Get(ctx)

	// Wait long enough for the second timer in timerCancellationSubWorkflow to fire
	workflow.Sleep(ctx, 20*time.Second)

	return err
}

func timerCancellationSubWorkflow(ctx workflow.Context) error {
	tctx, cancel := workflow.WithCancel(ctx)

	t := workflow.ScheduleTimer(ctx, 2*time.Second)
	t2 := workflow.ScheduleTimer(tctx, 10*time.Second)

	workflow.Select(
		ctx,
		workflow.Await(t, func(ctx workflow.Context, f workflow.Future[struct{}]) {
			// Cancel t2
			cancel()
		}),
		workflow.Await(t2, func(ctx workflow.Context, f workflow.Future[struct{}]) {
			// do nothing here, should never fire
			panic("timer should have been cancelled")
		}),
	)

	return nil
}

func Test_TimerRespondingWithoutNewEvents(t *testing.T) {
	tester := NewWorkflowTester[time.Time](workflowTimerRespondingWithoutNewEvents)

	tester.ScheduleCallback(time.Duration(2*time.Second), func() {
		tester.SignalWorkflow("signal", "s42")
	})

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())

	_, err := tester.WorkflowResult()
	require.Empty(t, err)
}

func workflowTimerRespondingWithoutNewEvents(ctx workflow.Context) error {
	workflow.ScheduleTimer(ctx, 1*time.Second).Get(ctx)

	workflow.Select(
		ctx,
		workflow.Receive(workflow.NewSignalChannel[any](ctx, "signal"), func(ctx workflow.Context, signal any, ok bool) {
			// do nothing
		}),
	)

	return nil
}

func Test_WallClockTimer_Canceled(t *testing.T) {
	activity1 := func() (string, error) {
		return "activity", nil
	}

	wf := func(ctx workflow.Context) (string, error) {
		tctx, cancel := workflow.WithCancel(ctx)
		defer cancel()

		workflow.ScheduleTimer(tctx, time.Millisecond*50)
		return workflow.ExecuteActivity[string](ctx, workflow.DefaultActivityOptions, activity1).Get(ctx)
	}

	tester := NewWorkflowTester[string](wf, WithTestTimeout(time.Second*3))

	tester.OnActivity(activity1).Return("activity", nil)

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	wr, _ := tester.WorkflowResult()
	require.Equal(t, "activity", wr)
	tester.AssertExpectations(t)
}

func Test_Timers_SetsTimeModeCorrectly(t *testing.T) {
	activity1 := func() error {
		return nil
	}

	wf := func(ctx workflow.Context) error {
		tctx, cancel := workflow.WithCancel(ctx)

		// This will be executed in wall-clock mode
		workflow.ScheduleTimer(tctx, time.Millisecond*50)
		workflow.ExecuteActivity[string](ctx, workflow.DefaultActivityOptions, activity1).Get(ctx)
		cancel()

		// This will switch to time-travel
		tctx, cancel = workflow.WithCancel(ctx)
		workflow.ScheduleTimer(tctx, time.Hour*24).Get(ctx)

		return nil
	}

	dl := newDebugLogger()

	tester := NewWorkflowTester[string](wf, WithTestTimeout(time.Second*10), WithLogger(dl.logger))

	tester.OnActivity(activity1).Return("activity", nil)

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	_, werr := tester.WorkflowResult()
	require.Empty(t, werr)
	tester.AssertExpectations(t)

	require.True(t, dl.hasLine("workflows.timer.mode.from=WallClock workflows.timer.mode.to=TimeTravel"))
	require.True(t, dl.hasLine("workflows.timer.mode.from=TimeTravel workflows.timer.mode.to=WallClock"))
}

func Test_Timers_MultipleTimers(t *testing.T) {
	activity1 := func() error {
		return nil
	}

	wf := func(ctx workflow.Context) error {
		for i := 0; i < 10; i++ {

			tctx, cancel := workflow.WithCancel(ctx)
			workflow.ScheduleTimer(tctx, time.Millisecond*10)
			workflow.ExecuteActivity[string](ctx, workflow.DefaultActivityOptions, activity1).Get(ctx)

			cancel()
		}

		return nil
	}

	dl := newDebugLogger()

	tester := NewWorkflowTester[string](wf, WithTestTimeout(time.Second*3), WithLogger(dl.logger))

	tester.OnActivity(activity1).Return("activity", nil)

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	_, werr := tester.WorkflowResult()
	require.Empty(t, werr)
	tester.AssertExpectations(t)
}
