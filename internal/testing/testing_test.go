package testing

import (
	"context"
	"fmt"
	"testing"

	"github.com/cschleiden/go-dt/pkg/workflow"
	"github.com/stretchr/testify/require"
)

func Test_ExecuteWorkflow(t *testing.T) {
	tester := NewWorkflowTester(WorkflowWithoutActivity)

	err := tester.Execute()

	require.NoError(t, err)
	require.True(t, tester.WorkflowFinished())
	tester.AssertExpectations(t)
}

func Test_ExecuteWorkflowWithActivity(t *testing.T) {
	tester := NewWorkflowTester(WorkflowWithActivity)

	tester.OnActivity(Activity1).Return(42, nil)

	err := tester.Execute()

	require.NoError(t, err)
	require.True(t, tester.WorkflowFinished())
	tester.AssertExpectations(t)
}

// func Test_ExecuteWorkflowWithActivity_Assertiongs(t *testing.T) {
// 	tester := NewWorkflowTester(WorkflowWithActivity)

// 	tester.OnActivity(Activity1).Times(2).Return(42, nil)

// 	err := tester.Execute(WorkflowWithActivity)

// 	require.NoError(t, err)
// 	require.True(t, tester.WorkflowFinished())

// 	tester.AssertExpectations(t)
// }

func Test_ExecuteWorkflowWithActivity_WithoutMock(t *testing.T) {
	tester := NewWorkflowTester(WorkflowWithActivity)

	require.Panics(t, func() {
		tester.Execute()
	})
}

func WorkflowWithoutActivity(ctx workflow.Context) error {
	return nil
}

func WorkflowWithActivity(ctx workflow.Context) error {
	var r int
	err := workflow.ExecuteActivity(ctx, workflow.DefaultActivityOptions, Activity1).Get(ctx, &r)
	if err != nil {
		return err
	}

	fmt.Println(r)

	return nil
}

func Activity1(ctx context.Context) (int, error) {
	panic("should not be called")
}
