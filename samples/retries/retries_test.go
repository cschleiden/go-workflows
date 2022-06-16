package main

import (
	"testing"

	"github.com/cschleiden/go-workflows/tester"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_Workflow(t *testing.T) {
	tester := tester.NewWorkflowTester[any](Workflow1)

	tester.Registry().RegisterWorkflow(WorkflowWithFailures)

	tester.OnActivity(Activity1, mock.Anything, mock.Anything).Return(42, nil).Once()

	tester.Execute("Hello world" + uuid.NewString())

	require.True(t, tester.WorkflowFinished())

	wr, werr := tester.WorkflowResult()
	require.Empty(t, wr)
	require.Empty(t, werr)
	tester.AssertExpectations(t)
}
