package tester

import (
	"errors"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/pkg/core"
	"github.com/cschleiden/go-workflows/pkg/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_SubWorkflow(t *testing.T) {
	subWorkflow := func(ctx workflow.Context, input string) (string, error) {
		return input + "sresult", nil
	}

	workflowWithSub := func(ctx workflow.Context, input string) (string, error) {
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

	tester := NewWorkflowTester(workflowWithSub)
	tester.Registry().RegisterWorkflow(subWorkflow)

	tester.Execute("hello")

	require.True(t, tester.WorkflowFinished())

	var wfR string
	tester.WorkflowResult(&wfR, nil)
	require.Equal(t, "hellosresult", wfR)
	tester.AssertExpectations(t)
}

func Test_SubWorkflow_Mocked(t *testing.T) {
	subWorkflow := func(ctx workflow.Context, input string) (string, error) {
		panic("should not call this")
	}

	workflow := func(ctx workflow.Context, input string) (string, error) {
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

	tester := NewWorkflowTester(workflow)
	tester.Registry().RegisterWorkflow(subWorkflow)
	tester.OnSubWorkflow(subWorkflow, mock.Anything, mock.Anything).Return("sresult2", nil)

	tester.Execute("hello")

	require.True(t, tester.WorkflowFinished())

	var wfR string
	tester.WorkflowResult(&wfR, nil)
	require.Equal(t, "sresult2", wfR)
	tester.AssertExpectations(t)
}

func Test_SubWorkflow_Mocked_Failure(t *testing.T) {
	subWorkflow := func(ctx workflow.Context, input string) (string, error) {
		panic("should not call this")
	}

	workflow := func(ctx workflow.Context, input string) (string, error) {
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

	tester := NewWorkflowTester(workflow)
	tester.Registry().RegisterWorkflow(subWorkflow)
	tester.OnSubWorkflow(subWorkflow, mock.Anything, mock.Anything).Return(nil, errors.New("error"))

	tester.Execute("hello")

	require.True(t, tester.WorkflowFinished())

	var wfR string
	var wfE string
	tester.WorkflowResult(&wfR, &wfE)
	require.Equal(t, "", wfR)
	require.Equal(t, "error", wfE)

	tester.AssertExpectations(t)
}

func Test_SubWorkflow_Signals(t *testing.T) {
	subWorkflow := func(ctx workflow.Context, input string) (string, error) {
		c := workflow.NewSignalChannel(ctx, "subworkflow-signal")
		var r string
		c.Receive(ctx, &r)

		return input + r, nil
	}

	workflowWithSub := func(ctx workflow.Context, input string) (string, error) {
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

	tester := NewWorkflowTester(workflowWithSub)
	tester.Registry().RegisterWorkflow(subWorkflow)

	var subWorkflowInstance core.WorkflowInstance

	tester.ListenSubWorkflow(func(instance core.WorkflowInstance, name string) {
		subWorkflowInstance = instance
	})

	tester.ScheduleCallback(time.Millisecond, func() {
		tester.SignalWorkflowInstance(subWorkflowInstance, "subworkflow-signal", "42")
	})

	tester.Execute("hello")

	require.True(t, tester.WorkflowFinished())

	var wfR string
	tester.WorkflowResult(&wfR, nil)
	require.Equal(t, "hello42", wfR)
	tester.AssertExpectations(t)
}
