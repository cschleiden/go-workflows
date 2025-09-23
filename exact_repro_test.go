package main

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/tester"
	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

func TestExactRepro(t *testing.T) {
	wft := tester.NewWorkflowTester[any](WFExact)
	// NOTE: Not registering the activity - this might be key!
	wft.Execute(context.Background())
	require.True(t, wft.WorkflowFinished())
	_, err := wft.WorkflowResult()
	require.ErrorContains(t, err, "workflow completed, but there are still pending futures")
}

func WFExact(ctx wf.Context) error {
	wf.ExecuteActivity[any](ctx, wf.DefaultActivityOptions, ActExact)
	return nil
}

func ActExact(context.Context) error {
	time.Sleep(10 * time.Second)
	return nil
}