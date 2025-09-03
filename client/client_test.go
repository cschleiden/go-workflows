package client

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/metrics"
	"github.com/cschleiden/go-workflows/workflow"
)

func Test_Client_CreateWorkflowInstance_ParamMismatch(t *testing.T) {
	wf := func(workflow.Context, int) (int, error) {
		return 0, nil
	}

	ctx := context.Background()

	b := &backend.MockBackend{}
	c := &Client{
		backend: b,
		clock:   clock.New(),
	}

	result, err := c.CreateWorkflowInstance(ctx, WorkflowInstanceOptions{
		InstanceID: "id",
	}, wf, "foo")
	require.Zero(t, result)
	require.EqualError(t, err, "mismatched argument type: expected int, got string")
	b.AssertExpectations(t)
}

func Test_Client_CreateWorkflowInstance_NameGiven(t *testing.T) {
	ctx := context.Background()

	b := &backend.MockBackend{}
	b.On("Options").Return(backend.ApplyOptions(backend.WithConverter(converter.DefaultConverter), backend.WithLogger(slog.Default())))
	b.On("Tracer").Return(noop.NewTracerProvider().Tracer("test"))
	b.On("Metrics").Return(metrics.NewNoopMetricsClient())
	b.On("CreateWorkflowInstance", mock.Anything, mock.Anything, mock.MatchedBy(func(event *history.Event) bool {
		if event.Type != history.EventType_WorkflowExecutionStarted {
			return false
		}

		a := event.Attributes.(*history.ExecutionStartedAttributes)

		return a.Name == "workflowName"
	})).Return(nil, nil)

	c := &Client{
		backend: b,
		clock:   clock.New(),
	}

	result, err := c.CreateWorkflowInstance(ctx, WorkflowInstanceOptions{
		InstanceID: "id",
	}, "workflowName", "foo")
	require.NoError(t, err)
	require.NotZero(t, result)
	b.AssertExpectations(t)
}

func Test_Client_GetWorkflowResultTimeout(t *testing.T) {
	instance := core.NewWorkflowInstance(uuid.NewString(), "test")

	ctx := context.Background()

	b := &backend.MockBackend{}
	b.On("Tracer").Return(noop.NewTracerProvider().Tracer("test"))
	b.On("GetWorkflowInstanceState", mock.Anything, instance).Return(core.WorkflowInstanceStateActive, nil)

	c := &Client{
		backend: b,
		clock:   clock.New(),
	}

	result, err := GetWorkflowResult[int](ctx, c, instance, time.Microsecond*1)
	require.Zero(t, result)
	require.EqualError(t, err, "workflow did not finish in time: workflow did not finish in specified timeout")
	b.AssertExpectations(t)
}

func Test_Client_GetWorkflowResultSuccess(t *testing.T) {
	instance := core.NewWorkflowInstance(uuid.NewString(), "test")

	ctx := context.Background()

	mockClock := clock.NewMock()

	r, _ := converter.DefaultConverter.To(42)

	b := &backend.MockBackend{}
	b.On("Tracer").Return(noop.NewTracerProvider().Tracer("test"))
	b.On("GetWorkflowInstanceState", mock.Anything, instance).Return(core.WorkflowInstanceStateActive, nil).Once().Run(func(args mock.Arguments) {
		// After the first call, advance the clock to immediately go to the second call below
		mockClock.Add(time.Second)
	})
	b.On("GetWorkflowInstanceState", mock.Anything, instance).Return(core.WorkflowInstanceStateFinished, nil)
	b.On("GetWorkflowInstanceHistory", mock.Anything, instance, (*int64)(nil)).Return([]*history.Event{
		history.NewHistoryEvent(1, time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
		history.NewHistoryEvent(2, time.Now(), history.EventType_WorkflowExecutionFinished, &history.ExecutionCompletedAttributes{
			Result: r,
			Error:  nil,
		}),
	}, nil)
	b.On("Options").Return(backend.ApplyOptions(backend.WithConverter(converter.DefaultConverter), backend.WithLogger(slog.Default())))

	c := &Client{
		backend: b,
		clock:   mockClock,
	}

	result, err := GetWorkflowResult[int](ctx, c, instance, 0)
	require.Equal(t, 42, result)
	require.NoError(t, err)
	b.AssertExpectations(t)
}

func Test_Client_SignalWorkflow(t *testing.T) {
	instanceID := uuid.NewString()

	ctx := context.Background()

	b := &backend.MockBackend{}
	b.On("Options").Return(backend.ApplyOptions(backend.WithConverter(converter.DefaultConverter), backend.WithLogger(slog.Default())))
	b.On("Tracer").Return(noop.NewTracerProvider().Tracer("test"))
	b.On("SignalWorkflow", mock.Anything, instanceID, mock.MatchedBy(func(event *history.Event) bool {
		return event.Type == history.EventType_SignalReceived &&
			event.Attributes.(*history.SignalReceivedAttributes).Name == "test"
	})).Return(nil)

	c := &Client{
		backend: b,
		clock:   clock.New(),
	}

	err := c.SignalWorkflow(ctx, instanceID, "test", "signal")

	require.NoError(t, err)
	b.AssertExpectations(t)
}

func Test_Client_SignalWorkflow_WithArgs(t *testing.T) {
	instanceID := uuid.NewString()

	ctx := context.Background()

	arg := 42

	input, _ := converter.DefaultConverter.To(arg)

	b := &backend.MockBackend{}
	b.On("Options").Return(backend.ApplyOptions(backend.WithConverter(converter.DefaultConverter), backend.WithLogger(slog.Default())))
	b.On("Tracer").Return(noop.NewTracerProvider().Tracer("test"))
	b.On("SignalWorkflow", mock.Anything, instanceID, mock.MatchedBy(func(event *history.Event) bool {
		return event.Type == history.EventType_SignalReceived &&
			event.Attributes.(*history.SignalReceivedAttributes).Name == "test" &&
			bytes.Equal(event.Attributes.(*history.SignalReceivedAttributes).Arg, input)
	})).Return(nil)

	c := &Client{
		backend: b,
		clock:   clock.New(),
	}

	err := c.SignalWorkflow(ctx, instanceID, "test", arg)

	require.NoError(t, err)
	b.AssertExpectations(t)
}
