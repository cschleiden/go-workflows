package main

import (
	"context"  
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/tester"
	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

// This is the CORRECT way - workflow waits for activity
func TestCorrectWorkflow(t *testing.T) {
	wft := tester.NewWorkflowTester[any](WFCorrect)
	wft.Registry().RegisterActivity(ActCorrect)
	wft.Execute(context.Background())
	require.True(t, wft.WorkflowFinished())
	_, err := wft.WorkflowResult()
	require.NoError(t, err) // Should be no error
}

func WFCorrect(ctx wf.Context) error {
	future := wf.ExecuteActivity[any](ctx, wf.DefaultActivityOptions, ActCorrect)
	_, err := future.Get(ctx) // WAIT for the activity to complete
	return err
}

// This is the INCORRECT way - workflow returns without waiting
func TestIncorrectWorkflow(t *testing.T) {
	wft := tester.NewWorkflowTester[any](WFIncorrect)
	wft.Registry().RegisterActivity(ActIncorrect)
	wft.Execute(context.Background())
	require.True(t, wft.WorkflowFinished())
	_, err := wft.WorkflowResult()
	require.ErrorContains(t, err, "workflow completed, but there are still pending futures")
}

func WFIncorrect(ctx wf.Context) error {
	wf.ExecuteActivity[any](ctx, wf.DefaultActivityOptions, ActIncorrect)
	return nil // BUG: Returns without waiting for activity
}

func ActCorrect(context.Context) error {
	time.Sleep(100 * time.Millisecond) 
	return nil
}

func ActIncorrect(context.Context) error {
	time.Sleep(100 * time.Millisecond)
	return nil
}