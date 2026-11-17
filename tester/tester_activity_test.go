package tester

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/activity"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_Activity_Long(t *testing.T) {
	activity1 := func() (string, error) {
		return "activity", nil
	}

	wf := func(ctx workflow.Context) (string, error) {
		tctx, cancel := workflow.WithCancel(ctx)

		var r string
		var err error

		workflow.Select(ctx,
			// Fire timer before `activity1` completes
			workflow.Await(workflow.ScheduleTimer(tctx, time.Millisecond*100), func(ctx workflow.Context, f workflow.Future[any]) {
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

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	wr, _ := tester.WorkflowResult()
	require.Equal(t, "timer", wr)
	tester.AssertExpectations(t)
}

func Test_Activity_RaceWithSignal(t *testing.T) {
	activity1 := func() (string, error) {
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

	tester.OnActivity(activity1).Run(func(args mock.Arguments) {
		time.Sleep(200 * time.Millisecond)
	}).Return("activity", nil)

	tester.ScheduleCallback(time.Millisecond*100, func() {
		tester.SignalWorkflow("signal", "stop")
	})

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	wr, _ := tester.WorkflowResult()
	require.Equal(t, "signal", wr)
	tester.AssertExpectations(t)
}

func Test_Activity_WithLogger(t *testing.T) {
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

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	tester.AssertExpectations(t)
}

func Test_Activity_Panic(t *testing.T) {
	activity1 := func(ctx context.Context) (string, error) {
		panic("activity panic")
	}

	wf := func(ctx workflow.Context) error {
		_, err := workflow.ExecuteActivity[string](ctx, workflow.DefaultActivityOptions, activity1).Get(ctx)
		return err
	}

	tester := NewWorkflowTester[string](wf, WithTestTimeout(time.Second*3))
	tester.Registry().RegisterActivity(activity1)
	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	var pe *workflow.PanicError
	_, err := tester.WorkflowResult()
	require.ErrorAs(t, err, &pe)
	tester.AssertExpectations(t)
}

func Test_ActivityCompletionAllowsTimeTravelBeforeProducerReturns(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)

	for _, tc := range []struct {
		name      string
		err       error
		eventType history.EventType
	}{
		{name: "success", eventType: history.EventType_ActivityCompleted},
		{name: "failure", err: errors.New("activity failed"), eventType: history.EventType_ActivityFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wt := NewWorkflowTester[any](func(workflow.Context) error { return nil })
			wt.callbacks = make(chan func() *history.WorkflowEvent)
			started := make(chan struct{})
			activity := func() error { return nil }
			wt.OnActivityByName("activity", activity).Run(func(mock.Arguments) {
				close(started)
			}).Return(tc.err).Once()

			event := history.NewPendingEvent(wt.Now(), history.EventType_ActivityScheduled,
				&history.ActivityScheduledAttributes{Name: "activity"})
			wt.scheduleActivity(wt.wfi, wt.wfm, event)
			<-started
			// Park the producer on the unbuffered send before consuming its result.
			runtime.Gosched()
			callback := <-wt.callbacks
			defer runtime.Gosched()

			completion := callback()
			require.Equal(t, tc.eventType, completion.HistoryEvent.Type)
			require.Equal(t, TM_TimeTravel, wt.newTimerMode())
		})
	}
}
