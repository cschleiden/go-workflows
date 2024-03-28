package executor

import (
	"context"
	"log"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/command"
	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/registry"
	wf "github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/goleak"
)

type testHistoryProvider struct {
	history []*history.Event
}

func (t *testHistoryProvider) GetWorkflowInstanceHistory(ctx context.Context, instance *core.WorkflowInstance, lastSequenceID *int64) ([]*history.Event, error) {
	return t.history, nil
}

func newExecutor(r *registry.Registry, i *core.WorkflowInstance, historyProvider WorkflowHistoryProvider) (*executor, error) {
	logger := slog.Default()
	tracer := trace.NewNoopTracerProvider().Tracer("test")

	e, err := NewExecutor(logger, tracer, r, converter.DefaultConverter, []wf.ContextPropagator{}, historyProvider, i, &metadata.WorkflowMetadata{}, clock.New())

	return e.(*executor), err
}

func activity1(ctx context.Context, r int) (int, error) {
	log.Println("Entering Activity1")
	return r, nil
}

func Test_Executor(t *testing.T) {
	tests := []struct {
		name string
		f    func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider)
	}{
		{
			name: "Simple_workflow_to_completion",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				workflowHits := 0
				wf := func(ctx sync.Context) error {
					workflowHits++
					return nil
				}

				r.RegisterWorkflow(wf)

				task := startWorkflowTask(i.InstanceID, wf)

				_, err := e.ExecuteTask(context.Background(), task)
				require.NoError(t, err)

				require.Equal(t, 1, workflowHits)
				require.True(t, e.workflow.Completed())
				require.Len(t, e.workflowState.Commands(), 1)
				require.IsType(t, &command.CompleteWorkflowCommand{}, e.workflowState.Commands()[0])
			},
		},
		{
			name: "Workflow with activity command",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				workflowActivityHit := 0
				workflowWithActivity := func(ctx sync.Context) error {
					workflowActivityHit++
					if _, err := wf.ExecuteActivity[int](ctx, wf.DefaultActivityOptions, activity1, 42).Get(ctx); err != nil {
						panic("error getting activity 1 result")
					}
					workflowActivityHit++
					return nil
				}

				r.RegisterWorkflow(workflowWithActivity)
				r.RegisterActivity(activity1)

				task := &backend.WorkflowTask{
					ID:               "taskID",
					WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
					Metadata:         &metadata.WorkflowMetadata{},
					NewEvents: []*history.Event{
						history.NewHistoryEvent(
							1,
							time.Now(),
							history.EventType_WorkflowExecutionStarted,
							&history.ExecutionStartedAttributes{
								Name:   fn.Name(workflowWithActivity),
								Inputs: []payload.Payload{},
							},
						),
					},
				}

				_, err := e.ExecuteTask(context.Background(), task)
				require.NoError(t, err)
				require.Nil(t, e.workflow.err)
				require.Equal(t, 1, workflowActivityHit)
				require.Len(t, e.workflowState.Commands(), 1)

				inputs, _ := converter.DefaultConverter.To(42)
				require.IsType(t, &command.ScheduleActivityCommand{}, e.workflowState.Commands()[0])
				require.Equal(t, command.CommandState_Committed, e.workflowState.Commands()[0].State())
				require.Equal(t, "activity1", e.workflowState.Commands()[0].(*command.ScheduleActivityCommand).Name)
				require.Equal(t, []payload.Payload{inputs}, e.workflowState.Commands()[0].(*command.ScheduleActivityCommand).Inputs)
			},
		},
		{
			name: "Workflow with activity replay",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				workflowActivityHit := 0
				workflowWithActivity := func(ctx sync.Context) error {
					workflowActivityHit++
					if _, err := wf.ExecuteActivity[int](ctx, wf.DefaultActivityOptions, activity1, 42).Get(ctx); err != nil {
						panic("error getting activity 1 result")
					}
					workflowActivityHit++
					return nil
				}

				r.RegisterWorkflow(workflowWithActivity)
				r.RegisterActivity(activity1)

				inputs, _ := converter.DefaultConverter.To(42)
				result, _ := converter.DefaultConverter.To(42)

				task := &backend.WorkflowTask{
					ID:               "taskID",
					WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
					Metadata:         &metadata.WorkflowMetadata{},
					LastSequenceID:   3,
				}

				hp.history = []*history.Event{
					history.NewHistoryEvent(
						1,
						time.Now(),
						history.EventType_WorkflowExecutionStarted,
						&history.ExecutionStartedAttributes{
							Name:   fn.Name(workflowWithActivity),
							Inputs: []payload.Payload{},
						},
					),
					history.NewHistoryEvent(
						2,
						time.Now(),
						history.EventType_ActivityScheduled,
						&history.ActivityScheduledAttributes{
							Name:   "activity1",
							Inputs: []payload.Payload{inputs},
						},
						history.ScheduleEventID(1),
					),
					history.NewHistoryEvent(
						3,
						time.Now(),
						history.EventType_ActivityCompleted,
						&history.ActivityCompletedAttributes{
							Result: result,
						},
						history.ScheduleEventID(1),
					),
				}

				_, err := e.ExecuteTask(context.Background(), task)
				require.NoError(t, err)
				require.Nil(t, e.workflow.err)
				require.Equal(t, 2, workflowActivityHit)
				require.True(t, e.workflow.Completed())
				require.Len(t, e.workflowState.Commands(), 2)
			},
		},
		{
			name: "Workflow with new events",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				workflowActivityHit := 0
				workflowWithActivity := func(ctx sync.Context) error {
					workflowActivityHit++
					if _, err := wf.ExecuteActivity[int](ctx, wf.DefaultActivityOptions, activity1, 42).Get(ctx); err != nil {
						panic("error getting activity 1 result")
					}
					workflowActivityHit++
					return nil
				}

				r.RegisterWorkflow(workflowWithActivity)
				r.RegisterActivity(activity1)

				inputs, _ := converter.DefaultConverter.To(42)
				result, _ := converter.DefaultConverter.To(42)

				oldTask := &backend.WorkflowTask{
					ID:               "oldtaskid",
					WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
					Metadata:         &metadata.WorkflowMetadata{},
					NewEvents: []*history.Event{
						history.NewPendingEvent(
							time.Now(),
							history.EventType_WorkflowExecutionStarted,
							&history.ExecutionStartedAttributes{
								Name:   fn.Name(workflowWithActivity),
								Inputs: []payload.Payload{},
							},
						),
						history.NewPendingEvent(
							time.Now(),
							history.EventType_ActivityScheduled,
							&history.ActivityScheduledAttributes{
								Name:   "activity1",
								Inputs: []payload.Payload{inputs},
							},
							history.ScheduleEventID(1),
						),
					},
				}

				taskResult, err := e.ExecuteTask(context.Background(), oldTask)
				require.NoError(t, err)
				require.Nil(t, e.workflow.err)
				require.Equal(t, 1, workflowActivityHit)
				require.False(t, e.workflow.Completed())
				require.Len(t, e.workflowState.Commands(), 1)

				newTask := &backend.WorkflowTask{
					ID:               "taskID",
					WorkflowInstance: oldTask.WorkflowInstance,
					Metadata:         &metadata.WorkflowMetadata{},
					NewEvents: []*history.Event{
						history.NewHistoryEvent(
							1,
							time.Now(),
							history.EventType_ActivityCompleted,
							&history.ActivityCompletedAttributes{
								Result: result,
							},
							history.ScheduleEventID(1),
						),
					},
					LastSequenceID: taskResult.Executed[len(taskResult.Executed)-1].SequenceID,
				}

				// Execute the workflow again with the activity completed event
				_, err = e.ExecuteTask(context.Background(), newTask)
				require.NoError(t, err)
				require.Nil(t, e.workflow.err)
				require.Equal(t, 2, workflowActivityHit)
				require.True(t, e.workflow.Completed())
				require.Len(t, e.workflowState.Commands(), 2)
			},
		},
		{
			name: "Workflow with selector",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				var workflowWithSelectorHits int

				workflowWithSelector := func(ctx sync.Context) error {
					workflowWithSelectorHits++

					f1 := wf.ExecuteActivity[int](ctx, wf.DefaultActivityOptions, activity1, 42)
					t := wf.ScheduleTimer(ctx, time.Millisecond*2)

					sync.Select(
						ctx,
						sync.Await[int](f1, func(ctx sync.Context, f sync.Future[int]) {
							workflowWithSelectorHits++
						}),
						sync.Await[any](t, func(ctx sync.Context, _ sync.Future[any]) {
							workflowWithSelectorHits++
						}),
					)

					workflowWithSelectorHits++

					return nil
				}

				r.RegisterWorkflow(workflowWithSelector)
				r.RegisterActivity(activity1)

				task := &backend.WorkflowTask{
					ID:               "taskID",
					WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
					Metadata:         &metadata.WorkflowMetadata{},
					NewEvents: []*history.Event{
						history.NewHistoryEvent(
							1,
							time.Now(),
							history.EventType_WorkflowExecutionStarted,
							&history.ExecutionStartedAttributes{
								Name:   fn.Name(workflowWithSelector),
								Inputs: []payload.Payload{},
							},
						),
					},
				}

				_, err := e.ExecuteTask(context.Background(), task)
				require.NoError(t, err)
				require.Nil(t, e.workflow.err)
				require.Equal(t, 1, workflowWithSelectorHits)
				require.Len(t, e.workflowState.Commands(), 2)

				require.IsType(t, &command.ScheduleActivityCommand{}, e.workflowState.Commands()[0])
				require.IsType(t, &command.ScheduleTimerCommand{}, e.workflowState.Commands()[1])
			},
		},
		{
			name: "Workflow with timer",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				workflowTimerHits := 0

				workflowWithTimer := func(ctx sync.Context) error {
					workflowTimerHits++

					if _, err := wf.ScheduleTimer(ctx, time.Millisecond*5).Get(ctx); err != nil {
						panic("error getting timer future")
					}

					workflowTimerHits++

					return nil
				}

				r.RegisterWorkflow(workflowWithTimer)

				task := &backend.WorkflowTask{
					ID:               "taskID",
					WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
					Metadata:         &metadata.WorkflowMetadata{},
					NewEvents: []*history.Event{
						history.NewHistoryEvent(
							1,
							time.Now(),
							history.EventType_WorkflowExecutionStarted,
							&history.ExecutionStartedAttributes{
								Name:   fn.Name(workflowWithTimer),
								Inputs: []payload.Payload{},
							},
						),
					},
				}

				_, err := e.ExecuteTask(context.Background(), task)
				require.NoError(t, err)
				require.Nil(t, e.workflow.err)
				require.Equal(t, 1, workflowTimerHits)
				require.Len(t, e.workflowState.Commands(), 1)

				require.Equal(t, int64(1), e.workflowState.Commands()[0].ID())
				require.IsType(t, &command.ScheduleTimerCommand{}, e.workflowState.Commands()[0])
			},
		},
		{
			name: "Cancel timer multiple times",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				workflowWithTimer := func(ctx sync.Context) error {
					tctx, cancel := wf.WithCancel(ctx)

					wf.ScheduleTimer(tctx, time.Millisecond*5)

					// Cause checkpoint
					wf.ExecuteActivity[any](ctx, wf.DefaultActivityOptions, activity1, 42).Get(ctx)

					cancel()
					cancel()

					return nil
				}

				r.RegisterWorkflow(workflowWithTimer)
				r.RegisterActivity(activity1)

				task := startWorkflowTask(i.InstanceID, workflowWithTimer)

				result, err := e.ExecuteTask(context.Background(), task)
				require.NoError(t, err)
				require.Nil(t, e.workflow.err)
				require.Len(t, e.workflowState.Commands(), 2)

				task2 := continueTask(i.InstanceID, []*history.Event{
					history.NewPendingEvent(time.Now(), history.EventType_ActivityCompleted, &history.ActivityCompletedAttributes{}, history.ScheduleEventID(2)),
				}, result.Executed[len(result.Executed)-1].SequenceID)

				_, err = e.ExecuteTask(context.Background(), task2)
				require.NoError(t, err)
				require.Nil(t, e.workflow.err)
			},
		},
		{
			name: "Workflow with signal",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				workflowSignalHits := 0

				workflowWithSignal := func(ctx sync.Context) error {
					c := wf.NewSignalChannel[string](ctx, "signal1")
					c.Receive(ctx)

					workflowSignalHits++

					return nil
				}

				r.RegisterWorkflow(workflowWithSignal)

				s, err := converter.DefaultConverter.To("")
				require.NoError(t, err)

				task := &backend.WorkflowTask{
					ID:               "taskID",
					WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
					Metadata:         &metadata.WorkflowMetadata{},
					NewEvents: []*history.Event{
						history.NewPendingEvent(
							time.Now(),
							history.EventType_WorkflowExecutionStarted,
							&history.ExecutionStartedAttributes{
								Name:   fn.Name(workflowWithSignal),
								Inputs: []payload.Payload{},
							},
						),
						history.NewPendingEvent(
							time.Now(),
							history.EventType_SignalReceived,
							&history.SignalReceivedAttributes{
								Name: "signal1",
								Arg:  s,
							},
						),
					},
				}

				_, err = e.ExecuteTask(context.Background(), task)
				require.NoError(t, err)
				require.Nil(t, e.workflow.err)
				require.Equal(t, 1, workflowSignalHits)
				require.True(t, e.workflow.Completed())
				require.Len(t, e.workflowState.Commands(), 1)
			},
		},
		{
			name: "Completes workflow on unhandled error",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				workflowPanic := func(ctx sync.Context) error {
					panic("wf error")
				}

				r.RegisterWorkflow(workflowPanic)

				task1 := &backend.WorkflowTask{
					ID:               "taskid",
					WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
					Metadata:         &metadata.WorkflowMetadata{},
					NewEvents: []*history.Event{
						history.NewPendingEvent(
							time.Now(),
							history.EventType_WorkflowExecutionStarted,
							&history.ExecutionStartedAttributes{
								Name:   fn.Name(workflowPanic),
								Inputs: []payload.Payload{},
							},
						),
					},
				}

				r1, err := e.ExecuteTask(context.Background(), task1)
				require.NoError(t, err)
				require.Error(t, e.workflow.err)
				require.True(t, e.workflow.Completed())
				require.Len(t, e.workflowState.Commands(), 1)
				require.Len(t, pendingCommands(e.workflowState.Commands()), 0)
				require.Equal(t, core.WorkflowInstanceStateFinished, r1.State)
			},
		},
		{
			name: "Schedule subworkflow",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				subworkflow := func(ctx wf.Context) error {
					return nil
				}

				workflow := func(ctx wf.Context) error {
					_, err := wf.CreateSubWorkflowInstance[any](ctx, wf.SubWorkflowOptions{
						InstanceID: "subworkflow",
					}, subworkflow).Get(ctx)

					return err
				}

				r.RegisterWorkflow(workflow)
				r.RegisterWorkflow(subworkflow)

				task := startWorkflowTask("instanceID", workflow)

				result, err := e.ExecuteTask(context.Background(), task)
				require.NoError(t, err)
				require.Nil(t, e.workflow.err)
				require.Len(t, result.Executed, 3)
				require.Len(t, result.WorkflowEvents, 1)
				require.Equal(t, history.EventType_WorkflowExecutionStarted, result.WorkflowEvents[0].HistoryEvent.Type)
			},
		},
		{
			name: "Schedule and cancel subworkflow",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				subworkflow := func(ctx wf.Context) error {
					return nil
				}

				workflow := func(ctx wf.Context) error {
					swctx, cancel := wf.WithCancel(ctx)

					f := wf.CreateSubWorkflowInstance[any](swctx, wf.SubWorkflowOptions{
						InstanceID: "subworkflow",
					}, subworkflow)

					wf.Sleep(ctx, time.Millisecond)

					cancel()

					f.Get(ctx)

					return nil
				}

				r.RegisterWorkflow(workflow)
				r.RegisterWorkflow(subworkflow)

				task := startWorkflowTask("instanceID", workflow)
				result, err := e.ExecuteTask(context.Background(), task)
				require.NoError(t, err)
				require.Nil(t, e.workflow.err)
				require.Len(t, result.Executed, 4)
				require.Len(t, result.TimerEvents, 1)
				require.Len(t, result.WorkflowEvents, 1)
				require.Equal(t, history.EventType_WorkflowExecutionStarted, result.WorkflowEvents[0].HistoryEvent.Type)

				subWorkflowInstance := result.WorkflowEvents[0].WorkflowInstance

				// Go past Sleep
				hp.history = append(hp.history, result.Executed...)
				result, err = e.ExecuteTask(context.Background(), continueTask("instanceID", []*history.Event{
					result.TimerEvents[0],
				}, result.Executed[len(result.Executed)-1].SequenceID))

				require.NoError(t, err)
				require.Len(t, result.WorkflowEvents, 1, "Cancellation should have been requested")
				require.Equal(t, history.EventType_WorkflowExecutionCanceled, result.WorkflowEvents[0].HistoryEvent.Type)
				require.Equal(
					t,
					subWorkflowInstance,
					result.WorkflowEvents[0].WorkflowInstance)

				require.Len(t, e.workflowState.Commands(), 2)

				// Complete subworkflow
				swr, _ := converter.DefaultConverter.To(nil)
				hp.history = append(hp.history, result.Executed...)
				_, err = e.ExecuteTask(context.Background(), continueTask("instanceID", []*history.Event{
					history.NewPendingEvent(time.Now(), history.EventType_SubWorkflowCompleted, &history.SubWorkflowCompletedAttributes{
						Result: swr,
					}, history.ScheduleEventID(1)),
				}, result.Executed[len(result.Executed)-1].SequenceID))

				require.NoError(t, err)
				require.True(t, e.workflow.Completed())
			},
		},
		{
			name: "Pending futures result in panic",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				subworkflow := func(ctx wf.Context) error {
					return nil
				}

				workflow := func(ctx wf.Context) error {
					// Start but not wait for sub-workflow
					wf.CreateSubWorkflowInstance[any](ctx, wf.SubWorkflowOptions{
						InstanceID: "subworkflow",
					}, subworkflow)

					// Schedule but not wait for timer
					wf.ScheduleTimer(ctx, time.Second*2)

					return nil
				}

				r.RegisterWorkflow(workflow)
				r.RegisterWorkflow(subworkflow)

				task := startWorkflowTask("instanceID", workflow)

				require.PanicsWithValue(t, "workflow completed, but there are still pending futures: [1-subworkflow:1 2-timer:2s]", func() {
					e.ExecuteTask(context.Background(), task)
				})
			},
		},
		{
			name: "Close_removes_any_goroutines",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				wf := func(ctx sync.Context) error {
					c := wf.NewSignalChannel[int](ctx, "signal")

					// Block workflow
					c.Receive(ctx)

					return nil
				}

				r.RegisterWorkflow(wf)

				task := startWorkflowTask(i.InstanceID, wf)

				goRoutines := runtime.NumGoroutine()

				_, err := e.ExecuteTask(context.Background(), task)
				require.NoError(t, err)

				require.Equal(t, goRoutines+1, runtime.NumGoroutine())

				e.Close()

				require.Equal(t, goRoutines, runtime.NumGoroutine())
			},
		},
		{
			name: "Close_removes_any_goroutines_defer",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				wf := func(ctx sync.Context) error {
					defer func() {
						_, err := wf.SignalWorkflow[any](ctx, "some-id", "signal", nil).Get(ctx)
						if err != nil {
							panic(err)
						}
					}()

					c := wf.NewSignalChannel[int](ctx, "signal")

					// Block workflow
					c.Receive(ctx)

					return nil
				}

				r.RegisterWorkflow(wf)

				task := startWorkflowTask(i.InstanceID, wf)

				_, err := e.ExecuteTask(context.Background(), task)
				require.NoError(t, err)

				e.Close()

				goleak.VerifyNone(t)
			},
		},
		{
			name: "Close_removes_any_goroutines_nested",
			f: func(t *testing.T, r *registry.Registry, e *executor, i *core.WorkflowInstance, hp *testHistoryProvider) {
				wf := func(ctx sync.Context) error {
					c := wf.NewSignalChannel[int](ctx, "signal")

					wf.Go(ctx, func(ctx wf.Context) {
						c.Receive(ctx)
					})

					// Block workflow
					c.Receive(ctx)

					return nil
				}

				r.RegisterWorkflow(wf)

				task := startWorkflowTask(i.InstanceID, wf)

				goRoutines := runtime.NumGoroutine()

				_, err := e.ExecuteTask(context.Background(), task)
				require.NoError(t, err)

				// Expect two pending goroutines
				require.Equal(t, goRoutines+2, runtime.NumGoroutine())

				e.Close()

				// Expect them to be removed
				require.Equal(t, goRoutines, runtime.NumGoroutine())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := registry.New()

			i := core.NewWorkflowInstance(uuid.NewString(), "")
			hp := &testHistoryProvider{}
			e, err := newExecutor(r, i, hp)
			require.NoError(t, err)
			tt.f(t, r, e, i, hp)

			e.Close()
		})
	}
}

func startWorkflowTask(instanceID string, workflow interface{}, workflowArgs ...interface{}) *backend.WorkflowTask {
	inputs, err := args.ArgsToInputs(converter.DefaultConverter, workflowArgs...)
	if err != nil {
		panic(err)
	}

	return &backend.WorkflowTask{
		ID:               uuid.NewString(),
		WorkflowInstance: core.NewWorkflowInstance(instanceID, "executionID"),
		Metadata:         &metadata.WorkflowMetadata{},
		NewEvents: []*history.Event{
			history.NewPendingEvent(
				time.Now(),
				history.EventType_WorkflowExecutionStarted,
				&history.ExecutionStartedAttributes{
					Name:   fn.Name(workflow),
					Inputs: inputs,
				},
			),
		},
	}
}

func continueTask(instanceID string, newEvents []*history.Event, lastSequenceID int64) *backend.WorkflowTask {
	return &backend.WorkflowTask{
		ID:               uuid.NewString(),
		WorkflowInstance: core.NewWorkflowInstance(instanceID, "executionID"),
		Metadata:         &metadata.WorkflowMetadata{},
		NewEvents:        newEvents,
		LastSequenceID:   lastSequenceID,
	}
}

func pendingCommands(commands []command.Command) []command.Command {
	var pending []command.Command
	for _, c := range commands {
		if c.State() == command.CommandState_Pending {
			pending = append(pending, c)
		}
	}
	return pending
}
