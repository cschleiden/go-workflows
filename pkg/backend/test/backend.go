package test

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/task"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type Tester struct {
	New func() backend.Backend
}

func TestBackend(t *testing.T, tester Tester) {
	s := new(BackendTestSuite)
	s.Tester = tester
	suite.Run(t, s)
}

type BackendTestSuite struct {
	suite.Suite

	Tester Tester
	b      backend.Backend
}

func (s *BackendTestSuite) SetupTest() {
	s.b = s.Tester.New()
}

func (s *BackendTestSuite) TestTester() {
	s.NotNil(s.b)
}

func (s *BackendTestSuite) TestGetWorkflowTaskReturnsNilWhenTimeout() {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond)
	defer cancel()

	task, err := s.b.GetWorkflowTask(ctx)
	s.Nil(task)
	s.NoError(err)
}

func (s *BackendTestSuite) TestGetActivityTaskReturnNilWhenTimeout() {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond)
	defer cancel()

	task, err := s.b.GetActivityTask(ctx)
	s.Nil(task)
	s.NoError(err)
}

func (s *BackendTestSuite) TestCreateWorkflowInstance_DoesNotError() {
	ctx := context.Background()

	err := s.b.CreateWorkflowInstance(ctx, core.TaskMessage{
		WorkflowInstance: core.NewWorkflowInstance(uuid.NewString(), ""),
		HistoryEvent:     history.NewHistoryEvent(history.EventType_WorkflowExecutionStarted, -1, &history.ExecutionStartedAttributes{}),
	})
	s.NoError(err)
}

func (s *BackendTestSuite) TestGetWorkflowTask_ReturnsTask() {
	ctx := context.Background()

	wfi := core.NewWorkflowInstance(uuid.NewString(), "")
	err := s.b.CreateWorkflowInstance(ctx, core.TaskMessage{
		WorkflowInstance: wfi,
		HistoryEvent:     history.NewHistoryEvent(history.EventType_WorkflowExecutionStarted, -1, &history.ExecutionStartedAttributes{}),
	})
	s.NoError(err)

	t, err := s.b.GetWorkflowTask(ctx)

	s.NoError(err)
	s.NotNil(t)
	s.Equal(wfi.GetInstanceID(), t.WorkflowInstance.GetInstanceID())
}

func (s *BackendTestSuite) TestGetWorkflowTask_LocksTask() {
	ctx := context.Background()

	wfi := core.NewWorkflowInstance(uuid.NewString(), "")
	err := s.b.CreateWorkflowInstance(ctx, core.TaskMessage{
		WorkflowInstance: wfi,
		HistoryEvent:     history.NewHistoryEvent(history.EventType_WorkflowExecutionStarted, -1, &history.ExecutionStartedAttributes{}),
	})
	s.Nil(err)

	// Get and lock only task
	t, err := s.b.GetWorkflowTask(ctx)
	s.NoError(err)
	s.NotNil(t)

	// First task is locked, second call should return nil
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond)
	defer cancel()

	t, err = s.b.GetWorkflowTask(ctx)

	s.NoError(err)
	s.Nil(t)
}

func (s *BackendTestSuite) TestCompleteWorkflowTask_ReturnsErrorIfNotLocked() {
	ctx := context.Background()

	wfi := core.NewWorkflowInstance(uuid.NewString(), "")
	err := s.b.CreateWorkflowInstance(ctx, core.TaskMessage{
		WorkflowInstance: wfi,
		HistoryEvent:     history.NewHistoryEvent(history.EventType_WorkflowExecutionStarted, -1, &history.ExecutionStartedAttributes{}),
	})
	s.NoError(err)

	//
	t := task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance(uuid.NewString(), ""),
		NewEvents:        []history.Event{},
	}

	err = s.b.CompleteWorkflowTask(ctx, t, []history.Event{}, []core.TaskMessage{})

	s.Error(err)
}
