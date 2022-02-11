package main

import (
	"testing"

	wtesting "github.com/cschleiden/go-dt/pkg/testing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_Workflow(t *testing.T) {
	tester := wtesting.NewWorkflowTester(Workflow1)

	tester.OnActivity(Activity1).Return(42)
	tester.OnActivity(Activity2).Return(12)

	// TODO: Timers?
	// TODO: Signals?

	err := tester.Execute("Hello world"+uuid.NewString(), 42, Inputs{
		Msg:   "",
		Times: 0,
	})

	require.NoError(t, err)
	require.True(t, tester.WorkflowFinished())
	tester.AssertExpectations(t)
}
