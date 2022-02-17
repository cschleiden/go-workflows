package testing

import (
	"context"
	"errors"
	"testing"

	"github.com/cschleiden/go-dt/pkg/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_ExecuteWorkflow(t *testing.T) {
	tester := NewWorkflowTester(workflowWithoutActivity)

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var wr int
	tester.WorkflowResult(&wr, nil)
	require.Equal(t, 0, wr)
	tester.AssertExpectations(t)
}

func Test_ExecuteWorkflowWithActivity(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	tester.OnActivity(activity1, mock.Anything).Return(42, nil)

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var wr int
	tester.WorkflowResult(&wr, nil)
	require.Equal(t, 42, wr)
	tester.AssertExpectations(t)
}

func Test_ExecuteWorkflowWithFailingActivity(t *testing.T) {
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

func Test_ExecuteWorkflowWithInvalidActivityMock(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	tester.OnActivity(activity1, mock.Anything).Return(1, 2, 3)

	require.PanicsWithValue(
		t,
		"Unexpected number of results returned for mocked activity activity1, expected 1 or 2, got 3",
		func() {
			tester.Execute()
		})
}

func Test_ExecuteWorkflowWithActivity_Retries(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	// Return two errors
	tester.OnActivity(activity1, mock.Anything).Return(0, errors.New("error")).Once()
	tester.OnActivity(activity1, mock.Anything).Return(42, nil)

	tester.Execute()

	var r int
	tester.WorkflowResult(&r, nil)
	require.Equal(t, 42, r)
}

func Test_ExecuteWorkflowWithActivity_WithoutMock(t *testing.T) {
	tester := NewWorkflowTester(workflowWithActivity)

	tester.Registry().RegisterActivity(activity1)

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
	}, activity1).Get(ctx, &r)
	if err != nil {
		return 0, err
	}

	return r, nil
}

func activity1(ctx context.Context) (int, error) {
	panic("should not be called")
}
