package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/cschleiden/go-workflows/tester"
)

func Test_Workflow(t *testing.T) {
	tester := tester.NewWorkflowTester[int](Workflow1)

	tester.OnSubWorkflowByName("SubWorkflowName", mock.Anything, "Hello world").Return(42, nil)

	tester.OnActivity(Activity1, mock.Anything, 35, 12).Return(47, nil)

	tester.Execute(context.Background(), "Hello world")

	require.True(t, tester.WorkflowFinished())

	wr, werr := tester.WorkflowResult()
	require.Equal(t, 47, wr)
	require.NoError(t, werr)
	tester.AssertExpectations(t)
}
