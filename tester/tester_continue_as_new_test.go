package tester

import (
	"context"
	"testing"

	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

func Test_ContinueAsNew(t *testing.T) {
	wf := func(ctx workflow.Context, run int) (int, error) {
		run = run + 1
		if run < 3 {
			return run, workflow.ContinueAsNew(ctx, run)
		}

		return run, nil
	}

	tester := NewWorkflowTester[int](wf)

	tester.Execute(context.Background(), 0)

	require.True(t, tester.WorkflowFinished())

	wfR, _ := tester.WorkflowResult()
	require.Equal(t, 3, wfR)
	tester.AssertExpectations(t)
}

func Test_ContinueAsNew_SubWorkflow(t *testing.T) {
	swf := func(ctx workflow.Context, run int) (int, error) {
		l := workflow.Logger(ctx)

		run = run + 1
		if run < 3 {
			l.Debug("continue as new", "run", run)
			return run, workflow.ContinueAsNew(ctx, run)
		}

		return run, nil
	}

	wf := func(ctx workflow.Context, run int) (int, error) {
		return workflow.CreateSubWorkflowInstance[int](ctx, workflow.DefaultSubWorkflowOptions, swf, run).Get(ctx)
	}

	tester := NewWorkflowTester[int](wf)

	tester.registry.RegisterWorkflow(swf)

	tester.Execute(context.Background(), 0)

	require.True(t, tester.WorkflowFinished())

	wfR, _ := tester.WorkflowResult()
	require.Equal(t, 3, wfR)
	tester.AssertExpectations(t)
}
