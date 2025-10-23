package tester

import (
	"context"
	"testing"

	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

func Test_InstanceExecutionDetails_HistoryLength(t *testing.T) {
	var capturedLength int64

	workflowWithInfo := func(ctx workflow.Context) error {
		info := workflow.InstanceExecutionDetails(ctx)
		capturedLength = info.HistoryLength
		return nil
	}

	tester := NewWorkflowTester[any](workflowWithInfo)
	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	require.Greater(t, capturedLength, int64(0), "History length should be greater than 0")
}

func Test_InstanceExecutionDetails_HistoryLength_WithActivity(t *testing.T) {
	var lengthBeforeActivity, lengthAfterActivity int64

	workflowWithActivity := func(ctx workflow.Context) (int, error) {
		info := workflow.InstanceExecutionDetails(ctx)
		lengthBeforeActivity = info.HistoryLength

		r, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, activity1).Get(ctx)
		if err != nil {
			return 0, err
		}

		info = workflow.InstanceExecutionDetails(ctx)
		lengthAfterActivity = info.HistoryLength

		return r, nil
	}

	tester := NewWorkflowTester[int](workflowWithActivity)
	tester.Registry().RegisterActivity(activity1)

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())
	require.Greater(t, lengthBeforeActivity, int64(0))
	require.Greater(t, lengthAfterActivity, lengthBeforeActivity, "History length should increase after activity execution")

	r, err := tester.WorkflowResult()
	require.NoError(t, err)
	require.Equal(t, 23, r)
}

func Test_InstanceExecutionDetails_HistoryLength_MultipleSteps(t *testing.T) {
	var lengths []int64

	workflowMultipleSteps := func(ctx workflow.Context) error {
		info := workflow.InstanceExecutionDetails(ctx)
		lengths = append(lengths, info.HistoryLength)

		workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, activity1).Get(ctx)

		info = workflow.InstanceExecutionDetails(ctx)
		lengths = append(lengths, info.HistoryLength)

		workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, activity1).Get(ctx)

		info = workflow.InstanceExecutionDetails(ctx)
		lengths = append(lengths, info.HistoryLength)

		return nil
	}

	tester := NewWorkflowTester[any](workflowMultipleSteps)
	tester.Registry().RegisterActivity(activity1)

	tester.Execute(context.Background())

	require.True(t, tester.WorkflowFinished())

	// The tester replays the workflow, so we'll see the lengths multiple times
	// We just need to verify that the final three captures show increasing values
	require.GreaterOrEqual(t, len(lengths), 3, "Should have at least 3 length captures")

	// Get the last 3 values (from the final execution)
	finalLengths := lengths[len(lengths)-3:]
	require.Greater(t, finalLengths[0], int64(0))
	require.Greater(t, finalLengths[1], finalLengths[0], "History should grow after first activity")
	require.Greater(t, finalLengths[2], finalLengths[1], "History should grow after second activity")
}
