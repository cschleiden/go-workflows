package tester

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/activity"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_LongActivity(t *testing.T) {
	activity1 := func() (string, error) {
		return "activity", nil
	}

	wf := func(ctx workflow.Context) (string, error) {
		tctx, cancel := workflow.WithCancel(ctx)

		var r string
		var err error

		workflow.Select(ctx,
			// Fire timer before `activity1` completes
			workflow.Await(workflow.ScheduleTimer(tctx, time.Millisecond*100), func(ctx workflow.Context, f workflow.Future[struct{}]) {
				// Timer fired
				r = "timer"
			}),
			workflow.Await(
				workflow.ExecuteActivity[string](
					ctx, workflow.DefaultActivityOptions, activity1),
				func(ctx workflow.Context, f workflow.Future[string]) {
					cancel() // cancel timer
					r, err = f.Get(ctx)
				}),
		)

		if err != nil {
			return "", err
		}

		return r, nil
	}

	tester := NewWorkflowTester[string](wf, WithTestTimeout(time.Second*3))

	tester.OnActivity(activity1).Run(func(args mock.Arguments) {
		time.Sleep(200 * time.Millisecond)
	}).Return("activity", nil)

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	wr, _ := tester.WorkflowResult()
	require.Equal(t, "timer", wr)
	tester.AssertExpectations(t)
}

func Test_ActivityRaceWithSignal(t *testing.T) {
	activity1 := func() (string, error) {
		time.Sleep(200 * time.Millisecond)

		return "activity", nil
	}

	wf := func(ctx workflow.Context) (string, error) {
		var r string
		var err error

		workflow.Select(ctx,
			workflow.Await(
				workflow.ExecuteActivity[string](
					ctx, workflow.DefaultActivityOptions, activity1),
				func(ctx workflow.Context, f workflow.Future[string]) {
					r, err = f.Get(ctx)
				}),
			workflow.Receive(
				workflow.NewSignalChannel[any](ctx, "signal"),
				func(ctx workflow.Context, signal any, ok bool) {
					r = "signal"
				}),
		)

		if err != nil {
			return "", err
		}

		return r, nil
	}

	tester := NewWorkflowTester[string](wf, WithTestTimeout(time.Second*3))

	tester.OnActivity(activity1).Return("activity", nil)

	tester.ScheduleCallback(time.Millisecond*100, func() {
		tester.SignalWorkflow("signal", "stop")
	})

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	wr, _ := tester.WorkflowResult()
	require.Equal(t, "signal", wr)
	tester.AssertExpectations(t)
}

func Test_ActivityWithLogger(t *testing.T) {
	activity1 := func(ctx context.Context) (string, error) {
		activity.Logger(ctx).Debug("hello from test")

		return "activity", nil
	}

	wf := func(ctx workflow.Context) error {
		workflow.ExecuteActivity[string](ctx, workflow.DefaultActivityOptions, activity1).Get(ctx)

		return nil
	}

	tester := NewWorkflowTester[string](wf, WithTestTimeout(time.Second*3))

	tester.Registry().RegisterActivity(activity1)

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	tester.AssertExpectations(t)
}
