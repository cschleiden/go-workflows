package tester

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_Workflow(t *testing.T) {
	workflowWithoutActivity := func(ctx workflow.Context) (int, error) {
		return 0, nil
	}

	tester := NewWorkflowTester[int](workflowWithoutActivity)

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	wr, _ := tester.WorkflowResult()
	require.Equal(t, 0, wr)
	tester.AssertExpectations(t)
}

func Test_WorkflowBlocked(t *testing.T) {
	tester := NewWorkflowTester[any](workflowBlocked, WithTestTimeout(time.Second*1))

	require.Panics(t, func() {
		tester.Execute(context.Background())
	})
}

func workflowBlocked(ctx workflow.Context) error {
	f := sync.NewFuture[int]()
	f.Get(ctx)

	return nil
}

func Test_Activity(t *testing.T) {
	tester := NewWorkflowTester[int](workflowWithActivity)

	tester.OnActivity(activity1, mock.Anything).Return(42, nil)

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	wr, _ := tester.WorkflowResult()
	require.Equal(t, 42, wr)
	tester.AssertExpectations(t)
}

func Test_OverrideActivity(t *testing.T) {
	tester := NewWorkflowTester[int](workflowWithActivity)

	tester.OnActivityByName("activity1", activity1, mock.Anything).Return(23, nil)

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	wr, _ := tester.WorkflowResult()
	require.Equal(t, 23, wr)
	tester.AssertExpectations(t)
}

func Test_FailingActivity(t *testing.T) {
	tester := NewWorkflowTester[int](workflowWithActivity)

	tester.OnActivity(activity1, mock.Anything).Return(0, errors.New("error"))

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	wr, werr := tester.WorkflowResult()
	require.Equal(t, 0, wr)
	require.EqualError(t, werr, "error")
	tester.AssertExpectations(t)
}

// func Test_InvalidActivityMock(t *testing.T) {
// 	tester := NewWorkflowTester[int](workflowWithActivity)

// 	tester.OnActivity(activityPanics, mock.Anything).Return(1, 2, 3)

// 	require.PanicsWithValue(
// 		t,
// 		"Unexpected number of results returned for mocked activity activityPanics, expected 1 or 2, got 3",
// 		func() {
// 			tester.Execute(context.Background())
// 		})
// }

func Test_Activity_Retries(t *testing.T) {
	tester := NewWorkflowTester[int](workflowWithActivity)

	// Return two errors
	tester.OnActivity(activity1, mock.Anything).Return(0, errors.New("error")).Once()
	tester.OnActivity(activity1, mock.Anything).Return(42, nil)

	tester.Execute(context.Background())

	r, _ := tester.WorkflowResult()
	require.Equal(t, 42, r)
}

func Test_Activity_WithoutMock(t *testing.T) {
	tester := NewWorkflowTester[int](workflowWithActivity)

	tester.Registry().RegisterActivity(activity1)

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	r, errStr := tester.WorkflowResult()
	require.Zero(t, errStr)
	require.Equal(t, 23, r)
	tester.AssertExpectations(t)
}

func workflowWithActivity(ctx workflow.Context) (int, error) {
	r, err := workflow.ExecuteActivity[int](ctx, workflow.ActivityOptions{
		RetryOptions: workflow.RetryOptions{
			MaxAttempts: 2,
		},
	}, activity1).Get(ctx)
	if err != nil {
		return 0, err
	}

	return r, nil
}

func activity1(ctx context.Context) (int, error) {
	return 23, nil
}

func Test_Activity_LongRunning(t *testing.T) {
	tester := NewWorkflowTester[any](workflowLongRunningActivity)
	tester.Registry().RegisterActivity(activityLongRunning)

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
}

func workflowLongRunningActivity(ctx workflow.Context) error {
	workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, activityLongRunning).Get(ctx)

	return nil
}

func activityLongRunning(ctx context.Context) (int, error) {
	time.Sleep(3 * time.Second)

	return 42, nil
}

func Test_CancelWorkflow(t *testing.T) {
	tester := NewWorkflowTester[any](func(ctx workflow.Context) error {
		_, _ = ctx.Done().Receive(ctx)
		return ctx.Err()
	})
	tester.ScheduleCallback(time.Duration(time.Second), func() {
		tester.CancelWorkflow()
	})

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	_, err := tester.WorkflowResult()
	require.EqualError(t, err, "context canceled")
}

func Test_Signals(t *testing.T) {
	tester := NewWorkflowTester[string](workflowSignal)
	tester.ScheduleCallback(time.Duration(5*time.Second), func() {
		tester.SignalWorkflow("signal", "s42")
	})

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())

	wfR, _ := tester.WorkflowResult()
	require.Equal(t, wfR, "s42")
	tester.AssertExpectations(t)
}

func workflowSignal(ctx workflow.Context) (string, error) {
	sc := workflow.NewSignalChannel[string](ctx, "signal")

	start := workflow.Now(ctx)

	val, ok := sc.Receive(ctx)
	if !ok {
		panic("channel should not be closed")
	}

	if workflow.Now(ctx).Sub(start) != 5*time.Second {
		return "", errors.New("delayed callback didn't fire at the right time")
	}

	return val, nil
}

func Test_SignalSubWorkflowBeforeScheduling(t *testing.T) {
	tester := NewWorkflowTester[string](workflowSubWorkFlowsAndSignals)

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	wfR, wfErr := tester.WorkflowResult()
	require.EqualError(t, wfErr, backend.ErrInstanceNotFound.Error())
	require.IsType(t, "", wfR)
}

func workflowSubWorkFlowsAndSignals(ctx workflow.Context) (string, error) {
	_, err := workflow.SignalWorkflow(ctx, "subworkflow", "test", "").Get(ctx)
	if err != backend.ErrInstanceNotFound {
		return "", err
	}

	return "finished without errors!", nil
}

func workflowSum(_ workflow.Context, valA, valB int) (int, error) {
	return valA + valB, nil
}

func Test_SignalSubWorkflow(t *testing.T) {
	tester := NewWorkflowTester[int](workflowSubworkflowSignal)
	require.NoError(t, tester.Registry().RegisterWorkflow(waitForSignal))

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	wfR, wfErr := tester.WorkflowResult()
	require.Empty(t, wfErr)
	require.Equal(t, 42, wfR)
}

func workflowSubworkflowSignal(ctx workflow.Context) (int, error) {
	sw := workflow.CreateSubWorkflowInstance[int](ctx, workflow.SubWorkflowOptions{
		InstanceID: "subworkflow",
	}, waitForSignal)

	_, err := workflow.SignalWorkflow(ctx, "subworkflow", "signal", "").Get(ctx)
	if err != nil {
		return 0, err
	}

	// Wait for subworkflow and return result
	return sw.Get(ctx)
}

func waitForSignal(ctx workflow.Context) (int, error) {
	workflow.Select(
		ctx,
		workflow.Receive(workflow.NewSignalChannel[any](ctx, "signal"), func(ctx workflow.Context, signal any, ok bool) {
			// Do nothing
		}),
	)

	return 42, nil
}

func Test_WithInitialTime(t *testing.T) {
	getTimeWorkflow := func(ctx workflow.Context) (time.Time, error) {
		return workflow.Now(ctx), nil
	}

	initialTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	tester := NewWorkflowTester[time.Time](getTimeWorkflow, WithInitialTime(initialTime))

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	r, _ := tester.WorkflowResult()
	require.Equal(t, initialTime, r)
}
