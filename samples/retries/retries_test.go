package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/cschleiden/go-workflows/tester"
)

func Test_Workflow(t *testing.T) {
	tester := tester.NewWorkflowTester[any](Workflow1)

	tester.Registry().RegisterWorkflow(WorkflowWithFailures)

	tester.OnActivity(Activity1, mock.Anything, mock.Anything).Return(42, nil).Once()

	tester.Execute(context.Background(), "Hello world"+uuid.NewString())

	require.True(t, tester.WorkflowFinished())

	wr, werr := tester.WorkflowResult()
	require.Empty(t, wr)
	require.NoError(t, werr)
	tester.AssertExpectations(t)
}
