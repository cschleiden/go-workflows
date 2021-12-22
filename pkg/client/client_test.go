package client

import (
	"bytes"
	"context"
	"testing"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/converter"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_Client_SignalWorkflow(t *testing.T) {
	instanceID := uuid.NewString()
	executionID := uuid.NewString()
	wfi := core.NewWorkflowInstance(instanceID, executionID)

	ctx := context.Background()

	b := &backend.MockBackend{}
	b.On("SignalWorkflow", ctx, wfi, mock.MatchedBy(func(event history.HistoryEvent) bool {
		return event.EventType == history.HistoryEventType_SignalReceived &&
			event.Attributes.(history.SignalReceivedAttributes).Name == "test"
	})).Return(nil)

	c := &client{
		backend: b,
	}

	err := c.SignalWorkflow(ctx, wfi, "test")

	require.Nil(t, err)
	b.AssertExpectations(t)
}

func Test_Client_SignalWorkflow_WithArgs(t *testing.T) {
	instanceID := uuid.NewString()
	executionID := uuid.NewString()
	wfi := core.NewWorkflowInstance(instanceID, executionID)

	ctx := context.Background()

	arg := 42

	input, _ := converter.DefaultConverter.To(arg)

	b := &backend.MockBackend{}
	b.On("SignalWorkflow", ctx, wfi, mock.MatchedBy(func(event history.HistoryEvent) bool {
		return event.EventType == history.HistoryEventType_SignalReceived &&
			event.Attributes.(history.SignalReceivedAttributes).Name == "test" &&
			bytes.Equal(event.Attributes.(history.SignalReceivedAttributes).Args[0], input)
	})).Return(nil)

	c := &client{
		backend: b,
	}

	err := c.SignalWorkflow(ctx, wfi, "test", arg)

	require.Nil(t, err)
	b.AssertExpectations(t)
}
