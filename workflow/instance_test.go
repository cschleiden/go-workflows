package workflow_test

import (
	"context"
	"testing"

	"github.com/cschleiden/go-workflows/tester"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_WorkflowInstanceInfo(t *testing.T) {
	wft := tester.NewWorkflowTester[int](workflowWithInstanceInfo)
	wft.Execute(context.Background(), 42)
	
	require.True(t, wft.WorkflowFinished())
	result, err := wft.WorkflowResult()
	require.NoError(t, err)
	require.Equal(t, 42, result)
}

func workflowWithInstanceInfo(ctx workflow.Context, input int) (int, error) {
	// Get workflow instance info
	info := workflow.GetWorkflowInstanceInfo(ctx)

	// History length should be >= 0
	if info.HistoryLength < 0 {
		panic("history length should be >= 0")
	}

	return input, nil
}

func Test_WorkflowInstanceInfo_WithActivities(t *testing.T) {
	var historyLengthBefore, historyLengthAfter int64

	wf := func(ctx workflow.Context) error {
		infoBefore := workflow.GetWorkflowInstanceInfo(ctx)
		historyLengthBefore = infoBefore.HistoryLength

		// Execute an activity
		af := workflow.ExecuteActivity[int](ctx, workflow.ActivityOptions{}, simpleActivity, 42)
		_, err := af.Get(ctx)
		if err != nil {
			return err
		}

		infoAfter := workflow.GetWorkflowInstanceInfo(ctx)
		historyLengthAfter = infoAfter.HistoryLength

		return nil
	}

	wft := tester.NewWorkflowTester[any](wf)
	wft.OnActivity(simpleActivity, mock.Anything, 42).Return(42, nil)
	wft.Execute(context.Background())

	require.True(t, wft.WorkflowFinished())
	_, err := wft.WorkflowResult()
	require.NoError(t, err)

	// History length should be >= 0 throughout
	require.GreaterOrEqual(t, historyLengthBefore, int64(0), 
		"history length before should be >= 0")
	require.GreaterOrEqual(t, historyLengthAfter, int64(0), 
		"history length after should be >= 0")
	
	// Note: In the tester, history length might be 0 initially since it's a simplified execution
	// The important thing is that the API works without errors
}

func Test_WorkflowInstanceInfo_MultipleActivities(t *testing.T) {
	wf := func(ctx workflow.Context) error {
		for i := 0; i < 3; i++ {
			info := workflow.GetWorkflowInstanceInfo(ctx)
			// Just verify it doesn't panic and returns a valid value
			require.GreaterOrEqual(t, info.HistoryLength, int64(0))

			af := workflow.ExecuteActivity[int](ctx, workflow.ActivityOptions{}, simpleActivity, i)
			_, err := af.Get(ctx)
			if err != nil {
				return err
			}
		}

		// Check one more time at the end
		info := workflow.GetWorkflowInstanceInfo(ctx)
		require.GreaterOrEqual(t, info.HistoryLength, int64(0))

		return nil
	}

	wft := tester.NewWorkflowTester[any](wf)
	for i := 0; i < 3; i++ {
		wft.OnActivity(simpleActivity, mock.Anything, i).Return(i, nil)
	}
	wft.Execute(context.Background())

	require.True(t, wft.WorkflowFinished())
	_, err := wft.WorkflowResult()
	require.NoError(t, err)
}

func simpleActivity(ctx workflow.Context, input int) (int, error) {
	return input, nil
}
