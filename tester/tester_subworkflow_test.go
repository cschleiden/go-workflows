package tester

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_SubWorkflow(t *testing.T) {
	subWorkflow := func(ctx workflow.Context, input string) (string, error) {
		return input + "sresult", nil
	}

	workflowWithSub := func(ctx workflow.Context, input string) (string, error) {
		sresult, err := workflow.CreateSubWorkflowInstance[string](
			ctx,
			workflow.DefaultSubWorkflowOptions,
			subWorkflow,
			input,
		).Get(ctx)
		if err != nil {
			return "", err
		}

		return sresult, nil
	}

	tester := NewWorkflowTester[string](workflowWithSub)
	tester.Registry().RegisterWorkflow(subWorkflow, nil)

	tester.Execute(context.Background(), "hello")

	require.True(t, tester.WorkflowFinished())

	wfR, _ := tester.WorkflowResult()
	require.Equal(t, "hellosresult", wfR)
	tester.AssertExpectations(t)
}

func Test_SubWorkflow_Mocked(t *testing.T) {
	subWorkflow := func(ctx workflow.Context, input string) (string, error) {
		panic("should not call this")
	}

	workflow := func(ctx workflow.Context, input string) (string, error) {
		sresult, err := workflow.CreateSubWorkflowInstance[string](
			ctx,
			workflow.DefaultSubWorkflowOptions,
			subWorkflow,
			input,
		).Get(ctx)
		if err != nil {
			return "", err
		}

		return sresult, nil
	}

	tester := NewWorkflowTester[string](workflow)
	tester.Registry().RegisterWorkflow(subWorkflow, nil)
	tester.OnSubWorkflow(subWorkflow, mock.Anything, mock.Anything).Return("sresult2", nil)

	tester.Execute(context.Background(), "hello")

	require.True(t, tester.WorkflowFinished())

	wfR, _ := tester.WorkflowResult()
	require.Equal(t, "sresult2", wfR)
	tester.AssertExpectations(t)
}

func Test_SubWorkflow_Mocked_Failure(t *testing.T) {
	subWorkflow := func(ctx workflow.Context, input string) (string, error) {
		panic("should not call this")
	}

	workflow := func(ctx workflow.Context, input string) (string, error) {
		sresult, err := workflow.CreateSubWorkflowInstance[string](
			ctx,
			workflow.DefaultSubWorkflowOptions,
			subWorkflow,
			input,
		).Get(ctx)
		if err != nil {
			return "", err
		}

		return sresult, nil
	}

	tester := NewWorkflowTester[string](workflow)
	tester.Registry().RegisterWorkflow(subWorkflow, nil)
	tester.OnSubWorkflow(subWorkflow, mock.Anything, mock.Anything).Return(nil, errors.New("swf error"))

	tester.Execute(context.Background(), "hello")

	require.True(t, tester.WorkflowFinished())

	wfR, wfE := tester.WorkflowResult()
	require.Equal(t, "", wfR)
	require.EqualError(t, wfE, "swf error")

	tester.AssertExpectations(t)
}

func Test_SubWorkflow_Signals(t *testing.T) {
	subWorkflow := func(ctx workflow.Context, input string) (string, error) {
		c := workflow.NewSignalChannel[string](ctx, "subworkflow-signal")
		r, _ := c.Receive(ctx)

		return input + r, nil
	}

	workflowWithSub := func(ctx workflow.Context, input string) (string, error) {
		sresult, err := workflow.CreateSubWorkflowInstance[string](
			ctx,
			workflow.DefaultSubWorkflowOptions,
			subWorkflow,
			input,
		).Get(ctx)
		if err != nil {
			return "", err
		}

		return sresult, nil
	}

	tester := NewWorkflowTester[string](workflowWithSub)
	tester.Registry().RegisterWorkflow(subWorkflow, nil)

	var subWorkflowInstance *core.WorkflowInstance

	tester.ListenSubWorkflow(func(instance *core.WorkflowInstance, name string) {
		subWorkflowInstance = instance
	})

	tester.ScheduleCallback(time.Millisecond, func() {
		require.Nil(t, tester.SignalWorkflowInstance(subWorkflowInstance, "subworkflow-signal", "42"))
	})

	tester.Execute(context.Background(), "hello")

	require.True(t, tester.WorkflowFinished())

	wfR, _ := tester.WorkflowResult()
	require.Equal(t, "hello42", wfR)
	tester.AssertExpectations(t)
}
