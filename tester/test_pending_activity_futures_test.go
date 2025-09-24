package tester

import (
	"context"
	"testing"
	"time"

	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

func TestFoo(t *testing.T) {
	wft := NewWorkflowTester[any](WF)
	// wft.OnActivity(Act, mock.Anything).Return(nil)
	wft.Execute(context.Background())
	require.True(t, wft.WorkflowFinished())
	_, err := wft.WorkflowResult()
	require.ErrorContains(t, err, "workflow completed, but there are still pending futures")
}

func WF(ctx wf.Context) error {
	wf.ExecuteActivity[any](ctx, wf.DefaultActivityOptions, Act)
	return nil
}

func Act(context.Context) error {
	time.Sleep(10 * time.Second)
	return nil
}