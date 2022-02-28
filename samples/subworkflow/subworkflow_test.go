package main

import (
	"testing"

	"github.com/cschleiden/go-workflows/internal/tester"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_Workflow(t *testing.T) {
	tester := tester.NewWorkflowTester(Workflow1)

	tester.Registry().RegisterWorkflow(Workflow2)

	tester.OnActivity(Activity1, mock.Anything, mock.Anything, mock.Anything).Return(47, nil)
	tester.OnActivity(Activity2, mock.Anything, mock.Anything, mock.Anything).Return(12, nil)

	tester.Execute("Hello world" + uuid.NewString())

	require.True(t, tester.WorkflowFinished())

	var wr int
	var werr string
	tester.WorkflowResult(&wr, &werr)
	require.Empty(t, wr)
	require.Empty(t, werr)
	tester.AssertExpectations(t)
}
