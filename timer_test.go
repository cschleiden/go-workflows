package main

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/tester"
	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

// Test with timer future instead of activity future
func TestIncorrectWorkflowTimer(t *testing.T) {
	wft := tester.NewWorkflowTester[any](WFIncorrectTimer)
	wft.Execute(context.Background())
	require.True(t, wft.WorkflowFinished())
	_, err := wft.WorkflowResult()
	require.ErrorContains(t, err, "workflow completed, but there are still pending futures")
}

func WFIncorrectTimer(ctx wf.Context) error {
	wf.ScheduleTimer(ctx, 10*time.Second) // Schedule timer but don't wait
	return nil // BUG: Returns without waiting for timer
}