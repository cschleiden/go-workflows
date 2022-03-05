package test

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/core/task"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type Tester struct {
	New func() backend.Backend

	Teardown func()
}

func TestBackend(t *testing.T, tester Tester) {
	s := new(BackendTestSuite)
	s.Tester = tester
	s.Assertions = require.New(t)
	suite.Run(t, s)
}

type BackendTestSuite struct {
	suite.Suite
	*require.Assertions

	Tester Tester
	b      backend.Backend
}

func (s *BackendTestSuite) SetupTest() {
	s.b = s.Tester.New()
}

func (s *BackendTestSuite) TearDownTest() {
	if s.Tester.Teardown != nil {
		s.Tester.Teardown()
	}
}

func (s *BackendTestSuite) TestTester() {
	s.NotNil(s.b)
}

func (s *BackendTestSuite) Test_GetWorkflowTask_ReturnsNilWhenTimeout() {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond)
	defer cancel()

	task, _ := s.b.GetWorkflowTask(ctx)
	s.Nil(task)
}

func (s *BackendTestSuite) Test_GetActivityTask_ReturnNilWhenTimeout() {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond)
	defer cancel()

	task, _ := s.b.GetActivityTask(ctx)
	s.Nil(task)
}

func (s *BackendTestSuite) Test_CreateWorkflowInstance_DoesNotError() {
	ctx := context.Background()

	err := s.b.CreateWorkflowInstance(ctx, core.WorkflowEvent{
		WorkflowInstance: core.NewWorkflowInstance(uuid.NewString(), uuid.NewString()),
		HistoryEvent:     history.NewHistoryEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
	})
	s.NoError(err)
}

func (s *BackendTestSuite) Test_GetWorkflowTask_ReturnsTask() {
	ctx := context.Background()

	wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	err := s.b.CreateWorkflowInstance(ctx, core.WorkflowEvent{
		WorkflowInstance: wfi,
		HistoryEvent:     history.NewHistoryEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
	})
	s.NoError(err)

	t, err := s.b.GetWorkflowTask(ctx)

	s.NoError(err)
	s.NotNil(t)
	s.Equal(wfi.GetInstanceID(), t.WorkflowInstance.GetInstanceID())
}

func (s *BackendTestSuite) Test_GetWorkflowTask_LocksTask() {
	ctx := context.Background()

	wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	err := s.b.CreateWorkflowInstance(ctx, core.WorkflowEvent{
		WorkflowInstance: wfi,
		HistoryEvent:     history.NewHistoryEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
	})
	s.Nil(err)

	// Get and lock only task
	t, err := s.b.GetWorkflowTask(ctx)
	s.NoError(err)
	s.NotNil(t)

	// First task is locked, second call should return nil
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond*10)
	defer cancel()

	t, err = s.b.GetWorkflowTask(ctx)

	s.NoError(err)
	s.Nil(t)
}

func (s *BackendTestSuite) Test_CompleteWorkflowTask_ReturnsErrorIfNotLocked() {
	ctx := context.Background()

	wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	err := s.b.CreateWorkflowInstance(ctx, core.WorkflowEvent{
		WorkflowInstance: wfi,
		HistoryEvent:     history.NewHistoryEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{}),
	})
	s.NoError(err)

	err = s.b.CompleteWorkflowTask(ctx, wfi, []history.Event{}, []core.WorkflowEvent{})

	s.Error(err)
}

func (s *BackendTestSuite) Test_CompleteWorkflowTask_AddsNewEventsToHistory() {
	ctx := context.Background()

	startedEvent := history.NewHistoryEvent(time.Now(), history.EventType_WorkflowExecutionStarted, &history.ExecutionStartedAttributes{})
	activityScheduledEvent := history.NewHistoryEvent(time.Now(), history.EventType_ActivityScheduled, &history.ActivityScheduledAttributes{}, history.ScheduleEventID(1))
	activityCompletedEvent := history.NewHistoryEvent(time.Now(), history.EventType_ActivityCompleted, &history.ActivityCompletedAttributes{}, history.ScheduleEventID(1))

	wfi := core.NewWorkflowInstance(uuid.NewString(), uuid.NewString())
	err := s.b.CreateWorkflowInstance(ctx, core.WorkflowEvent{
		WorkflowInstance: wfi,
		HistoryEvent:     startedEvent,
	})
	s.NoError(err)

	_, err = s.b.GetWorkflowTask(ctx)
	s.NoError(err)

	taskStartedEvent := history.NewHistoryEvent(time.Now(), history.EventType_WorkflowTaskStarted, &history.WorkflowTaskStartedAttributes{})
	taskFinishedEvent := history.NewHistoryEvent(time.Now(), history.EventType_WorkflowTaskFinished, &history.WorkflowTaskFinishedAttributes{})
	events := []history.Event{
		taskStartedEvent,
		startedEvent,
		activityScheduledEvent,
		taskFinishedEvent,
	}

	workflowEvents := []core.WorkflowEvent{
		{
			WorkflowInstance: wfi,
			HistoryEvent:     activityCompletedEvent,
		},
	}

	err = s.b.CompleteWorkflowTask(ctx, wfi, events, workflowEvents)
	s.NoError(err)

	time.Sleep(time.Second)

	t, err := s.b.GetWorkflowTask(ctx)
	s.NotEqual(task.Continuation, t.Kind, "Expect full task")
	s.NoError(err)
	s.NotNil(t)
	s.Equal(len(events), len(t.History))
	// Only compare event types
	for i, expected := range events {
		s.Equal(expected.Type, t.History[i].Type)
	}
	s.Len(t.NewEvents, 1)
	s.Equal(activityCompletedEvent.Type, t.NewEvents[0].Type, "Expected new events to be returned")
}
