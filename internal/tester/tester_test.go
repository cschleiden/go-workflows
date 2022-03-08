package tester

import (
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_Workflow(t *testing.T) {
	workflowWithoutActivity := func(ctx workflow.Context) (int, error) {
		return 0, nil
	}

	tester := NewWorkflowTester(workflowWithoutActivity)

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var wr int
	tester.WorkflowResult(&wr, nil)
	require.Equal(t, 0, wr)
	tester.AssertExpectations(t)
}

func Test_WorkflowBlocked(t *testing.T) {
	tester := NewWorkflowTester(workflowBlocked)

	require.Panics(t, func() {
		tester.Execute()
	})
}

func workflowBlocked(ctx workflow.Context) error {
	var f workflow.Future
	f.Get(ctx, nil)

	return nil
}

func Test_Activity(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	tester.OnActivity(activity1, mock.Anything).Return(42, nil)

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var wr int
	tester.WorkflowResult(&wr, nil)
	require.Equal(t, 42, wr)
	tester.AssertExpectations(t)
}

func Test_FailingActivity(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	tester.OnActivity(activity1, mock.Anything).Return(0, errors.New("error"))

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var wr int
	var werr string
	tester.WorkflowResult(&wr, &werr)
	require.Equal(t, 0, wr)
	require.Equal(t, "error", werr)
	tester.AssertExpectations(t)
}

// func Test_InvalidActivityMock(t *testing.T) {
// 	tester := NewWorkflowTester(workflowWithActivity)

// 	tester.OnActivity(activityPanics, mock.Anything).Return(1, 2, 3)

// 	require.PanicsWithValue(
// 		t,
// 		"Unexpected number of results returned for mocked activity activityPanics, expected 1 or 2, got 3",
// 		func() {
// 			tester.Execute()
// 		})
// }

func Test_Activity_Retries(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	// Return two errors
	tester.OnActivity(activity1, mock.Anything).Return(0, errors.New("error")).Once()
	tester.OnActivity(activity1, mock.Anything).Return(42, nil)

	tester.Execute()

	var r int
	tester.WorkflowResult(&r, nil)
	require.Equal(t, 42, r)
}

func Test_Activity_WithoutMock(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	tester.Registry().RegisterActivity(activity1)

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var r int
	var errStr string
	tester.WorkflowResult(&r, &errStr)
	require.Zero(t, errStr)
	require.Equal(t, 23, r)
	tester.AssertExpectations(t)
}

func workflowWithActivity(ctx workflow.Context) (int, error) {
	var r int
	err := workflow.ExecuteActivity(ctx, workflow.ActivityOptions{
		RetryOptions: workflow.RetryOptions{
			MaxAttempts: 2,
		},
	}, activity1).Get(ctx, &r)
	if err != nil {
		return 0, err
	}

	return r, nil
}

func activity1(ctx context.Context) (int, error) {
	return 23, nil
}

func Test_Activity_LongRunning(t *testing.T) {
	tester := NewWorkflowTester(workflowLongRunningActivity)
	tester.Registry().RegisterActivity(activityLongRunning)

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
}

func workflowLongRunningActivity(ctx workflow.Context) error {
	workflow.ExecuteActivity(ctx, workflow.DefaultActivityOptions, activityLongRunning).Get(ctx, nil)

	return nil
}

func activityLongRunning(ctx context.Context) (int, error) {
	time.Sleep(3 * time.Second)

	return 42, nil
}

func Test_Timer(t *testing.T) {
	tester := NewWorkflowTester(workflowTimer)
	start := tester.Now()

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var wr timerResult
	tester.WorkflowResult(&wr, nil)
	require.True(t, start.Equal(wr.T1))

	e := start.Add(30 * time.Second)
	require.True(t, e.Equal(wr.T2), "expected %v, got %v", e, wr.T2)
}

type timerResult struct {
	T1 time.Time
	T2 time.Time
}

func workflowTimer(ctx workflow.Context) (timerResult, error) {
	log.Println("workflowWithTimer-Before", workflow.Now(ctx))

	t1 := workflow.Now(ctx)

	workflow.ScheduleTimer(ctx, 30*time.Second).Get(ctx, nil)

	log.Println("workflowWithTimer-After", workflow.Now(ctx))

	t2 := workflow.Now(ctx)

	workflow.ScheduleTimer(ctx, 30*time.Second).Get(ctx, nil)

	return timerResult{
		T1: t1,
		T2: t2,
	}, nil
}

func Test_TimerCancellation(t *testing.T) {
	tester := NewWorkflowTester(workflowTimerCancellation)
	start := tester.Now()

	tester.Execute()

	require.True(t, tester.WorkflowFinished())

	var wfR time.Time
	tester.WorkflowResult(&wfR, nil)
	require.True(t, start.Equal(wfR), "expected %v, got %v", start, wfR)
}

func workflowTimerCancellation(ctx workflow.Context) (time.Time, error) {
	tctx, cancel := workflow.WithCancel(ctx)
	t := workflow.ScheduleTimer(tctx, 30*time.Second)
	cancel()

	workflow.Select(ctx, workflow.Await(t, func(ctx workflow.Context, t workflow.Future) {
		t.Get(ctx, nil)
	}))

	return workflow.Now(ctx), nil
}

func Test_Signals(t *testing.T) {
	tester := NewWorkflowTester(workflowSignal)
	tester.ScheduleCallback(time.Duration(5*time.Second), func() {
		tester.SignalWorkflow("signal", "s42")
	})

	tester.Execute()

	require.True(t, tester.WorkflowFinished())

	var wfR string
	tester.WorkflowResult(&wfR, nil)
	require.Equal(t, wfR, "s42")
	tester.AssertExpectations(t)
}

func workflowSignal(ctx workflow.Context) (string, error) {
	sc := workflow.NewSignalChannel(ctx, "signal")

	start := workflow.Now(ctx)

	var val string
	more := sc.Receive(ctx, &val)
	if more != true {
		panic("channel should not be closed")
	}

	if workflow.Now(ctx).Sub(start) != 5*time.Second {
		return "", errors.New("delayed callback didn't fire at the right time")
	}

	return val, nil
}
