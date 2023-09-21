package test

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	internalwf "github.com/cschleiden/go-workflows/internal/workflow"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type backendTest struct {
	name         string
	options      []backend.BackendOption
	withoutCache bool // If set, test will only be run when the cache is disabled
	f            func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend)
}

func EndToEndBackendTest(t *testing.T, setup func(options ...backend.BackendOption) TestBackend, teardown func(b TestBackend)) {
	tests := []backendTest{
		{
			name: "SimpleWorkflow",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				wf := func(ctx workflow.Context, msg string) (string, error) {
					return msg + " world", nil
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				output, err := runWorkflowWithResult[string](t, ctx, c, wf, "hello")

				require.NoError(t, err)
				require.Equal(t, "hello world", output)
			},
		},
		{
			name: "SimpleWorkflow_ExpectedHistory",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				wf := func(ctx workflow.Context, msg string) (string, error) {
					return msg + " world", nil
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				instance := runWorkflow(t, ctx, c, wf, "hello")

				require.NoError(t, c.WaitForWorkflowInstance(ctx, instance, time.Second*10))

				events, err := b.GetWorkflowInstanceHistory(ctx, instance, nil)
				require.NoError(t, err)

				require.Equal(t, history.EventType_WorkflowTaskStarted, events[0].Type)
				require.Equal(t, history.EventType_WorkflowExecutionStarted, events[1].Type)
				require.Equal(t, int64(0), events[1].ScheduleEventID)
				require.Equal(t, history.EventType_WorkflowExecutionFinished, events[2].Type)
				require.Equal(t, int64(0), events[2].ScheduleEventID)
			},
		},
		{
			name: "UnregisteredWorkflow_Errors",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				wf := func(ctx workflow.Context, msg string) (string, error) {
					return msg + " world", nil
				}
				register(t, ctx, w, nil, nil)

				output, err := runWorkflowWithResult[string](t, ctx, c, wf, "hello")

				require.Zero(t, output)
				require.ErrorContains(t, err, "workflow 1 not found")
			},
		},
		{
			name: "WorkflowArgumentMismatch",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				wf := func(ctx workflow.Context, p1 int) (int, error) {
					return 42, nil
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				instance, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
					InstanceID: uuid.NewString(),
				}, wf)

				require.Nil(t, instance)
				require.ErrorContains(t, err, "mismatched argument count: expected 1, got 0")
			},
		},
		{
			name: "UnregisteredActivity_Errors",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				a := func(context.Context) error { return nil }
				wf := func(ctx workflow.Context) (int, error) {
					return workflow.ExecuteActivity[int](ctx, workflow.ActivityOptions{
						RetryOptions: workflow.RetryOptions{
							MaxAttempts: 1,
						},
					}, a).Get(ctx)
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				output, err := runWorkflowWithResult[int](t, ctx, c, wf)

				require.Zero(t, output)
				require.ErrorContains(t, err, "activity not found")
			},
		},
		{
			name: "ActivityArgumentMismatch",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				a := func(context.Context, int, int) error { return nil }
				wf := func(ctx workflow.Context) (int, error) {
					return workflow.ExecuteActivity[int](ctx, workflow.ActivityOptions{
						RetryOptions: workflow.RetryOptions{
							MaxAttempts: 1,
						},
					}, a, 42).Get(ctx)
				}
				register(t, ctx, w, []interface{}{wf}, []interface{}{a})

				output, err := runWorkflowWithResult[int](t, ctx, c, wf)

				require.Zero(t, output)
				require.ErrorContains(t, err, "mismatched argument count: expected 2, got 1")
			},
		},
		{
			name: "SideEffect_Simple",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				i := 2
				wf := func(ctx workflow.Context) (int, error) {
					r1, _ := workflow.SideEffect(ctx, func(ctx workflow.Context) int {
						i++
						return i
					}).Get(ctx)

					// Do something to force the task to end
					workflow.Sleep(ctx, time.Millisecond*1)

					r2, _ := workflow.SideEffect(ctx, func(ctx workflow.Context) int {
						i++
						return i
					}).Get(ctx)

					return r1 + r2, nil
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				instance := runWorkflow(t, ctx, c, wf)

				r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*5)
				require.NoError(t, err)
				require.Equal(t, 7, r)
			},
		},
		{
			name: "Signal_after_completion",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				wf := func(ctx workflow.Context) error {
					return nil
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				// Run workflow to completion
				instance := runWorkflow(t, ctx, c, wf)
				_, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*20)
				require.NoError(t, err)

				err = c.SignalWorkflow(ctx, instance.InstanceID, "signal", nil)
				require.ErrorIs(t, err, backend.ErrInstanceNotFound)
			},
		},
		{
			name: "SubWorkflow_Simple",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				swf := func(ctx workflow.Context, i int) (int, error) {
					return i * 2, nil
				}
				wf := func(ctx workflow.Context) (int, error) {
					r, err := workflow.CreateSubWorkflowInstance[int](ctx, workflow.DefaultSubWorkflowOptions, swf, 1).Get(ctx)
					if err != nil {
						return 0, err
					}

					return r, nil
				}
				register(t, ctx, w, []interface{}{wf, swf}, nil)

				instance := runWorkflow(t, ctx, c, wf)

				r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*20)
				require.NoError(t, err)
				require.Equal(t, 2, r)
			},
		},
		{
			name: "SubWorkflow_PropagateCancellation",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				canceled := int32(0)

				swf := func(ctx workflow.Context, i int) (int, error) {
					// Sleep in this sub workflow, we expect the subworkflow to be canceled, so this timer will not complete.
					if err := workflow.Sleep(ctx, time.Second*10); err != nil && err != workflow.Canceled {
						// This should not happen
						return 0, err
					}

					if ctx.Err() != nil && ctx.Err() == workflow.Canceled {
						atomic.AddInt32(&canceled, 1)
					}

					return i * 2, nil
				}

				// Workflow will be executed multiple times, but the test will wait only once. Create buffered channel
				ch := make(chan struct{}, 10)

				wf := func(ctx workflow.Context) (int, error) {
					swfs := make([]workflow.Future[int], 0)

					swfs = append(swfs, workflow.CreateSubWorkflowInstance[int](ctx, workflow.DefaultSubWorkflowOptions, swf, 1))
					swfs = append(swfs, workflow.CreateSubWorkflowInstance[int](ctx, workflow.DefaultSubWorkflowOptions, swf, 2))

					// Unblock test. Should not do this in production code, but here we know that this will be executed in the same process.
					ch <- struct{}{}

					// Wait for subworkflows to complete
					r := 0

					for _, f := range swfs {
						sr, err := f.Get(ctx)
						if err != nil && err != workflow.Canceled {
							return 0, err
						}

						r = r + sr
					}

					if ctx.Err() != nil && ctx.Err() == workflow.Canceled {
						atomic.AddInt32(&canceled, 1)
					}

					return r, nil
				}
				register(t, ctx, w, []interface{}{wf, swf}, nil)

				instance := runWorkflow(t, ctx, c, wf)

				// Wait for the workflow to start running
				<-ch

				require.NoError(t, c.CancelWorkflowInstance(ctx, instance))

				r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*10)
				require.NoError(t, err)
				require.Equal(t, int32(3), canceled)
				require.Equal(t, 6, r)
			},
		},
		{
			name: "SubWorkflow_CancelBeforeStarting",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				swInstanceID := "subworkflow"

				swfrun := 0
				swf := func(ctx workflow.Context, i int) (int, error) {
					swfrun++
					return i * 2, nil
				}
				wf := func(ctx workflow.Context) (int, error) {
					swfctx, cancel := workflow.WithCancel(ctx)

					f := workflow.CreateSubWorkflowInstance[int](swfctx, workflow.SubWorkflowOptions{
						InstanceID: swInstanceID,
					}, swf, 1)

					// Cancel before it can be started
					cancel()

					// Force the checkpoint before continuing the execution
					workflow.Sleep(ctx, time.Millisecond*2)

					r, err := f.Get(ctx)
					if err != nil && err != workflow.Canceled {
						return 0, err
					}

					return r, nil
				}
				register(t, ctx, w, []interface{}{wf, swf}, nil)

				instance := runWorkflow(t, ctx, c, wf)
				r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*5)
				require.NoError(t, err)
				require.Equal(t, 0, r)
				require.Equal(t, 0, swfrun, "sub-workflow should not run")

				_, err = b.GetWorkflowInstanceState(ctx, &core.WorkflowInstance{
					InstanceID: swInstanceID,
				})

				require.Error(t, err)
				require.Equal(t, backend.ErrInstanceNotFound, err)
			},
		},
		{
			name: "SubWorkflow_Signal",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				swf := func(ctx workflow.Context, i int) (int, error) {
					workflow.NewSignalChannel[string](ctx, "signal").Receive(ctx)

					return i * 2, nil
				}
				wf := func(ctx workflow.Context) (int, error) {
					id, _ := workflow.SideEffect(ctx, func(ctx workflow.Context) string {
						id := uuid.New().String()
						workflow.Logger(ctx).Warn("side effect", "id", id)
						return id
					}).Get(ctx)

					f := workflow.CreateSubWorkflowInstance[int](ctx, workflow.SubWorkflowOptions{
						InstanceID: id,
					}, swf, 1)

					if _, err := workflow.SignalWorkflow(ctx, id, "signal", "hello").Get(ctx); err != nil {
						return 0, err
					}

					return f.Get(ctx)
				}
				register(t, ctx, w, []interface{}{wf, swf}, nil)

				instance := runWorkflow(t, ctx, c, wf)

				r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*20)
				require.NoError(t, err)
				require.Equal(t, 2, r)
			},
		},
		{
			name: "SubWorkflow_Signal_BeforeStarting",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				wf := func(ctx workflow.Context) (int, error) {
					id, _ := workflow.SideEffect(ctx, func(ctx workflow.Context) string {
						id := uuid.New().String()
						workflow.Logger(ctx).Warn("side effect", "id", id)
						return id
					}).Get(ctx)

					if _, err := workflow.SignalWorkflow(ctx, id, "signal", "hello").Get(ctx); err != nil {
						return 0, err
					}

					return 42, nil
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				instance := runWorkflow(t, ctx, c, wf)

				_, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*20)
				require.ErrorContains(t, err, backend.ErrInstanceNotFound.Error())
			},
		},
		{
			name: "Timer_CancelWorkflowInstance",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				a := func(ctx context.Context) error {
					return nil
				}
				wf := func(ctx workflow.Context) error {
					_, err := workflow.ScheduleTimer(ctx, time.Second*10).Get(ctx)
					if err != nil && err != workflow.Canceled {
						return err
					}

					return nil
				}
				register(t, ctx, w, []interface{}{wf}, []interface{}{a})

				instance := runWorkflow(t, ctx, c, wf)

				// Allow some time for the timer to get scheduled
				time.Sleep(time.Millisecond * 200)

				require.NoError(t, c.CancelWorkflowInstance(ctx, instance))

				_, err := client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
				require.NoError(t, err)
			},
		},
		{
			name: "Timer_CancelBeforeStarting",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				a := func(ctx context.Context) error {
					return nil
				}
				wf := func(ctx workflow.Context) error {
					tctx, cancel := workflow.WithCancel(ctx)

					f := workflow.ScheduleTimer(tctx, time.Second*10)

					// Cancel before it can be started
					cancel()

					// Force the checkpoint before continuing the execution
					workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

					_, err := f.Get(ctx)
					if err != nil && err != workflow.Canceled {
						return err
					}

					return nil
				}
				register(t, ctx, w, []interface{}{wf}, []interface{}{a})

				instance := runWorkflow(t, ctx, c, wf)
				_, err := client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
				require.NoError(t, err)

				events, err := b.GetWorkflowInstanceHistory(ctx, instance, nil)
				require.NoError(t, err)
				for _, e := range events {
					if e.Type == history.EventType_TimerScheduled {
						require.FailNow(t, "timer should not be scheduled")
					}
				}

				futureEvents, err := b.GetFutureEvents(ctx)
				require.NoError(t, err)
				require.Len(t, futureEvents, 0, "no future events should be scheduled")
			},
		},
		{
			name: "Timer_CancelTwice",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				a := func(ctx context.Context) error {
					return nil
				}
				wf := func(ctx workflow.Context) error {
					tctx, cancel := workflow.WithCancel(ctx)
					f := workflow.ScheduleTimer(tctx, time.Second*10)

					// Force the checkpoint before continuing the execution
					workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

					// Cancel timer
					cancel()

					// Force another checkpoint
					workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

					cancel()

					// Force another checkpoint
					workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

					if _, err := f.Get(ctx); err != nil && err != workflow.Canceled {
						return err
					}

					return nil
				}
				register(t, ctx, w, []interface{}{wf}, []interface{}{a})

				instance := runWorkflow(t, ctx, c, wf)
				_, err := client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
				require.NoError(t, err)

				historyContains(ctx, t, b, instance, history.EventType_TimerScheduled, history.EventType_TimerCanceled)

				futureEvents, err := b.GetFutureEvents(ctx)
				require.NoError(t, err)
				require.Len(t, futureEvents, 0, "no future events should be scheduled")
			},
		},
		{
			name: "Timer_CancelBeforeFiringRemovesFutureEvent",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				a := func(ctx context.Context) error {
					return nil
				}
				wf := func(ctx workflow.Context) error {
					tctx, cancel := workflow.WithCancel(ctx)
					f := workflow.ScheduleTimer(tctx, time.Second*10)

					// Force the checkpoint before continuing the execution
					workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

					// Cancel timer
					cancel()

					// Force another checkpoint
					workflow.ExecuteActivity[any](ctx, workflow.DefaultActivityOptions, a).Get(ctx)

					if _, err := f.Get(ctx); err != nil && err != workflow.Canceled {
						return err
					}

					return nil
				}
				register(t, ctx, w, []interface{}{wf}, []interface{}{a})

				instance := runWorkflow(t, ctx, c, wf)
				_, err := client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
				require.NoError(t, err)

				historyContains(ctx, t, b, instance, history.EventType_TimerScheduled, history.EventType_TimerCanceled)

				futureEvents, err := b.GetFutureEvents(ctx)
				require.NoError(t, err)
				require.Len(t, futureEvents, 0, "no future events should be scheduled")
			},
		},
		{
			name:         "NonDeterminism",
			withoutCache: true,
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				i := 0
				wf := func(ctx workflow.Context) (int, error) {
					var r int

					i++
					if i%2 == 0 {
						r, _ = workflow.SideEffect(ctx, func(ctx workflow.Context) int {
							return 1
						}).Get(ctx)
					} else {
						workflow.Sleep(ctx, time.Millisecond*1)
					}

					// Do something to force the task to end
					workflow.Sleep(ctx, time.Millisecond*1)

					return r, nil
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				instance := runWorkflow(t, ctx, c, wf)

				r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*5)
				require.NoError(t, err)
				require.Equal(t, 0, r)
			},
		},
		{
			name: "RemoveWorkflowInstance",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				wf := func(ctx workflow.Context, msg string) (string, error) {
					return msg + " world", nil
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				instance := runWorkflow(t, ctx, c, wf, "hello")

				r, err := client.GetWorkflowResult[string](ctx, c, instance, time.Second*5)
				require.NoError(t, err)
				require.Equal(t, "hello world", r)

				err = c.RemoveWorkflowInstance(ctx, instance)
				require.NoError(t, err)

				_, err = client.GetWorkflowResult[string](ctx, c, instance, time.Second*5)
				require.Error(t, err)
				require.ErrorIs(t, err, backend.ErrInstanceNotFound)
			},
		},
		{
			name:    "ContextPropagation",
			options: []backend.BackendOption{backend.WithContextPropagator(&testContextPropagator{})},
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				a := func(ctx context.Context) (int, error) {
					d := myValues(ctx)
					return d.Count, nil
				}

				wf := func(ctx workflow.Context, msg string) (string, error) {
					// Get values from context
					d := myValuesWf(ctx)

					// Update context before calling activity
					ctx = withMyValuesWf(ctx, &myData{Name: d.Name, Count: d.Count - 19})

					ar, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, a).Get(ctx)
					if err != nil {
						return "", err
					}

					return fmt.Sprintf("%s-%d", d.Name, ar), nil
				}

				register(t, ctx, w, []interface{}{wf}, []interface{}{a})

				ctx = withMyValues(ctx, &myData{Name: "hello", Count: 42})

				instance := runWorkflow(t, ctx, c, wf, "hello")

				r, err := client.GetWorkflowResult[string](ctx, c, instance, time.Second*5)
				require.NoError(t, err)
				require.Equal(t, "hello-23", r)
			},
		},
		{
			name: "ContinueAsNew",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				wf := func(ctx workflow.Context, run int) (int, error) {
					run = run + 1
					if run < 3 {
						return run, workflow.ContinueAsNew(ctx, run)
					}

					return run, nil
				}
				register(t, ctx, w, []interface{}{wf}, nil)

				instance := runWorkflow(t, ctx, c, wf, 0)

				r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*10)
				require.NoError(t, err)
				require.Equal(t, 1, r)

				state, err := b.GetWorkflowInstanceState(ctx, instance)
				require.NoError(t, err)
				require.Equal(t, core.WorkflowInstanceStateContinuedAsNew, state)
			},
		},
		{
			name: "ContinueAsNew_Subworkflow",
			f: func(t *testing.T, ctx context.Context, c client.Client, w worker.Worker, b TestBackend) {
				swf := func(ctx workflow.Context, run int) (int, error) {
					l := workflow.Logger(ctx)

					run = run + 1
					if run < 3 {
						l.Debug("continue as new", "run", run)
						return run, workflow.ContinueAsNew(ctx, run)
					}

					return run, nil
				}

				wf := func(ctx workflow.Context, run int) (int, error) {
					return workflow.CreateSubWorkflowInstance[int](ctx, workflow.DefaultSubWorkflowOptions, swf, run).Get(ctx)
				}
				register(t, ctx, w, []interface{}{wf, swf}, nil)

				instance := runWorkflow(t, ctx, c, wf, 0)

				r, err := client.GetWorkflowResult[int](ctx, c, instance, time.Second*20)
				require.NoError(t, err)
				require.Equal(t, 3, r)
			},
		},
	}

	tests = append(tests, e2eActivityTests...)
	tests = append(tests, e2eStatsTests...)

	run := func(suffix string, workerOptions *worker.Options) {
		for _, tt := range tests {
			if tt.withoutCache && workerOptions.WorkflowExecutorCache != nil {
				// Skip test
				continue
			}

			t.Run(tt.name+suffix, func(t *testing.T) {
				b := setup(tt.options...)
				ctx := context.Background()
				ctx, cancel := context.WithCancel(ctx)

				c := client.New(b)
				w := worker.New(b, workerOptions)

				t.Cleanup(func() {
					cancel()

					// Wait for in-progress executions to finish
					if err := w.WaitForCompletion(); err != nil {
						log.Println("Worker did not stop in time")
						t.FailNow()
					}

					if teardown != nil {
						teardown(b)
					}
				})

				tt.f(t, ctx, c, w, b)
			})
		}
	}

	options := worker.DefaultWorkerOptions

	// Run with cache
	run("", &options)

	// Disable cache for this execution
	options.WorkflowExecutorCache = &noopWorkflowExecutorCache{}
	run("_without_cache", &options)
}

type noopWorkflowExecutorCache struct {
}

var _ internalwf.ExecutorCache = (*noopWorkflowExecutorCache)(nil)

// Get implements workflow.ExecutorCache
func (*noopWorkflowExecutorCache) Get(ctx context.Context, instance *core.WorkflowInstance) (internalwf.WorkflowExecutor, bool, error) {
	return nil, false, nil
}

// StartEviction implements workflow.ExecutorCache
func (*noopWorkflowExecutorCache) StartEviction(ctx context.Context) {
}

// Store implements workflow.ExecutorCache
func (*noopWorkflowExecutorCache) Store(ctx context.Context, instance *core.WorkflowInstance, workflow internalwf.WorkflowExecutor) error {
	return nil
}

func register(t *testing.T, ctx context.Context, w worker.Worker, workflows []interface{}, activities []interface{}) {
	for _, wf := range workflows {
		require.NoError(t, w.RegisterWorkflow(wf))
	}

	for _, a := range activities {
		require.NoError(t, w.RegisterActivity(a))
	}

	err := w.Start(ctx)
	require.NoError(t, err)
}

func runWorkflow(t *testing.T, ctx context.Context, c client.Client, wf interface{}, inputs ...interface{}) *workflow.Instance {
	instance, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, wf, inputs...)
	require.NoError(t, err)

	return instance
}

func runWorkflowWithResult[T any](t *testing.T, ctx context.Context, c client.Client, wf interface{}, inputs ...interface{}) (T, error) {
	instance := runWorkflow(t, ctx, c, wf, inputs...)
	return client.GetWorkflowResult[T](ctx, c, instance, time.Second*10)
}

func historyIterate(ctx context.Context, t *testing.T, b TestBackend, instance *workflow.Instance, f func(event *history.Event) bool) {
	events, err := b.GetWorkflowInstanceHistory(ctx, instance, nil)
	require.NoError(t, err)
	for _, e := range events {
		if !f(e) {
			break
		}
	}
}

// historyContains ensure the history contains all of the given event types in the given order
func historyContains(ctx context.Context, t *testing.T, b TestBackend, instance *workflow.Instance, eventTypes ...history.EventType) {
	historyIterate(ctx, t, b, instance, func(event *history.Event) bool {
		if len(eventTypes) == 0 {
			return false
		}

		if event.Type == eventTypes[0] {
			eventTypes = eventTypes[1:]
		}

		return true
	})

	require.Equal(t, []history.EventType{}, eventTypes, "history does not contain all event types")
}
