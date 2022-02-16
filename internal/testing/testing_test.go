package testing

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cschleiden/go-dt/pkg/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_ExecuteWorkflow(t *testing.T) {
	tester := NewWorkflowTester(WorkflowWithoutActivity)

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var wr int
	tester.WorkflowResult(&wr, nil)
	require.Equal(t, 0, wr)
	tester.AssertExpectations(t)
}

func Test_ExecuteWorkflowWithActivity(t *testing.T) {
	tester := NewWorkflowTester(WorkflowWithActivity)

	tester.OnActivity(Activity1, mock.Anything).Return(42, nil)

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var wr int
	tester.WorkflowResult(&wr, nil)
	require.Equal(t, 42, wr)
	tester.AssertExpectations(t)
}

func Test_ExecuteWorkflowWithFailingActivity(t *testing.T) {
	tester := NewWorkflowTester(WorkflowWithActivity)

	tester.OnActivity(Activity1, mock.Anything).Return(0, errors.New("error"))

	tester.Execute()

	require.True(t, tester.WorkflowFinished())
	var wr int
	var werr string
	tester.WorkflowResult(&wr, &werr)
	require.Equal(t, 0, wr)
	require.Equal(t, "", werr)
	tester.AssertExpectations(t)
}

func Test_ExecuteWorkflowWithInvalidActivityMock(t *testing.T) {
	tester := NewWorkflowTester(WorkflowWithActivity)

	tester.OnActivity(Activity1, mock.Anything).Return(1, 2, 3)

	require.PanicsWithValue(
		t,
		"Unexpected number of results returned for activity Activity1, expected 1 or 2, got 3",
		func() {
			tester.Execute()
		})
}

func Test_ExecuteWorkflowWithActivity_Retries(t *testing.T) {
	tester := NewWorkflowTester(WorkflowWithActivity)

	tester.OnActivity(Activity1, mock.Anything).Return(0, errors.New("error")).Twice()

	tester.Execute()

	var werr string
	tester.WorkflowResult(nil, &werr)
	require.Equal(t, "error", werr)
}

func Test_ExecuteWorkflowWithActivity_WithoutMock(t *testing.T) {
	tester := NewWorkflowTester(WorkflowWithActivity)

	tester.Registry().RegisterActivity(Activity1)

	require.PanicsWithValue(t, "should not be called", func() {
		tester.Execute()
	})
}

func WorkflowWithoutActivity(ctx workflow.Context) (int, error) {
	return 0, nil
}

func WorkflowWithActivity(ctx workflow.Context) (int, error) {
	var r int
	err := workflow.ExecuteActivity(ctx, workflow.ActivityOptions{
		RetryOptions: workflow.RetryOptions{
			MaxAttempts: 2,
		},
	}, Activity1).Get(ctx, &r)
	if err != nil {
		return 0, err
	}

	fmt.Println(r)

	return r, nil
}

func Activity1(ctx context.Context) (int, error) {
	panic("should not be called")
}
