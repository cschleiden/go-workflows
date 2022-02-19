package testing

import (
	"testing"

	"github.com/cschleiden/go-workflows/pkg/workflow"
	"github.com/stretchr/testify/require"
)

func Test_SubWorkflow(t *testing.T) {
	tester := NewWorkflowTester(workflowWithSub)
	tester.Registry().RegisterWorkflow(subWorkflow)

	tester.Execute("hello")

	require.True(t, tester.WorkflowFinished())

	var wfR string
	tester.WorkflowResult(&wfR, nil)
	require.Equal(t, wfR, "")
	tester.AssertExpectations(t)
}

func workflowWithSub(ctx workflow.Context, input string) (string, error) {
	var sresult string
	err := workflow.CreateSubWorkflowInstance(
		ctx,
		workflow.DefaultSubWorkflowOptions,
		subWorkflow,
		input,
	).Get(ctx, &sresult)
	if err != nil {
		return "", err
	}

	return sresult, nil
}

func subWorkflow(ctx workflow.Context, input string) (string, error) {
	return input + "sresult", nil
}
