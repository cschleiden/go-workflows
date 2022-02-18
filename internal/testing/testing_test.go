package testing

import (
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/pkg/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_Workflow(t *testing.T) {
	tester := NewWorkflowTester(workflowWithoutActivity)

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var wr int
	tester.WorkflowResult(&wr, nil)
	require.Equal(t, 0, wr)
	tester.AssertExpectations(t)
}

func Test_WorkflowWithActivity(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	tester.OnActivity(activityPanics, mock.Anything).Return(42, nil)

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var wr int
	tester.WorkflowResult(&wr, nil)
	require.Equal(t, 42, wr)
	tester.AssertExpectations(t)
}

func Test_WorkflowWithFailingActivity(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	tester.OnActivity(activityPanics, mock.Anything).Return(0, errors.New("error"))

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var wr int
	var werr string
	tester.WorkflowResult(&wr, &werr)
	require.Equal(t, 0, wr)
	require.Equal(t, "error", werr)
	tester.AssertExpectations(t)
}

func Test_WorkflowWithInvalidActivityMock(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	tester.OnActivity(activityPanics, mock.Anything).Return(1, 2, 3)

	require.PanicsWithValue(
		t,
		"Unexpected number of results returned for mocked activity activityPanics, expected 1 or 2, got 3",
		func() {
			tester.Execute()
		})
}

func Test_WorkflowWithActivity_Retries(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	// Return two errors
	tester.OnActivity(activityPanics, mock.Anything).Return(0, errors.New("error")).Once()
	tester.OnActivity(activityPanics, mock.Anything).Return(42, nil)

	tester.Execute()

	var r int
	tester.WorkflowResult(&r, nil)
	require.Equal(t, 42, r)
}

func Test_WorkflowWithActivity_WithoutMock(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	tester.Registry().RegisterActivity(activityPanics)

	require.PanicsWithValue(t, "should not be called", func() {
		tester.Execute()
	})
}

func workflowWithoutActivity(ctx workflow.Context) (int, error) {
	return 0, nil
}

func workflowWithActivity(ctx workflow.Context) (int, error) {
	var r int
	err := workflow.ExecuteActivity(ctx, workflow.ActivityOptions{
		RetryOptions: workflow.RetryOptions{
			MaxAttempts: 2,
		},
	}, activityPanics).Get(ctx, &r)
	if err != nil {
		return 0, err
	}

	return r, nil
}

func activityPanics(ctx context.Context) (int, error) {
	panic("should not be called")
}

func Test_WorkflowWithTimer(t *testing.T) {
	tester := NewWorkflowTester(workflowWithTimer)
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

func workflowWithTimer(ctx workflow.Context) (timerResult, error) {
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

func Test_WorkflowWithTimerCancellation(t *testing.T) {
	tester := NewWorkflowTester(workflowWithCancellation)
	start := tester.Now()

	tester.Execute()

	require.True(t, tester.WorkflowFinished())

	var wfR time.Time
	tester.WorkflowResult(&wfR, nil)
	require.True(t, start.Equal(wfR), "expected %v, got %v", start, wfR)
}

func workflowWithCancellation(ctx workflow.Context) (time.Time, error) {
	tctx, cancel := workflow.WithCancel(ctx)
	t := workflow.ScheduleTimer(tctx, 30*time.Second)
	cancel()

	s := workflow.NewSelector()
	s.AddFuture(t, func(ctx workflow.Context, t workflow.Future) {
		t.Get(ctx, nil)
	})
	s.Select(ctx)

	return workflow.Now(ctx), nil
}
