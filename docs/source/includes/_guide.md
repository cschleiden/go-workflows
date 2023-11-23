# Guide

## Registering workflows

```go
func Workflow1(ctx workflow.Context) error {
}
```

Workflows need to accept `workflow.Context` as their first parameter, and any number of inputs parameters afterwards. Parameters need to be serializable (e.g., no `chan`s etc.). Workflows need to return an `error` and optionally one additional result, which again needs to be serializable.

```go
var b backend.Backend
w := worker.New(b)

w.RegisterWorkflow(Workflow1)
```

Workflows needs to be registered with the worker before they can be started.

## Registering activities

```go
func Activity1(ctx context.Context, a, b int) (int, error) {
	return a + b, nil
}

var b backend.Backend
w := worker.New(b)

w.RegisterActivity(Activity1)
```

Similar to workflows, activities need to be registered with the worker before they can be started. They also need to accept `context.Context` as their first parameter, and any number of inputs parameters afterwards. Parameters need to be serializable (e.g., no `chan`s etc.). Activities need to return an `error` and optionally one additional result, which again needs to be serializable.

Activites can be registered as plain `func`s or as methods on a struct. The latter is useful if you want to provide some shared state to activities, for example, a database connection.

And using a `struct`:

```go
type act struct {
	SharedState int
}

func (a *act) Activity1(ctx context.Context, a, b int) (int, error) {
	return a + b + act.SharedState, nil
}

func (a *act) Activity2(ctx context.Context, a int) (int, error) {
	return a * act.SharedState, nil
}

var b backend.Backend
w := worker.New(b)

w.RegisterActivity(&act{SharedState: 12})
```

to call activities registered on a struct from a workflow:

```go
// ...
var a *act

r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, a.Activity1, 35, 12).Get(ctx)
if err != nil {
	// handle error
}

// Output r1 = 47 + 12 (from the worker registration) = 59
```

## Starting workflows

`CreateWorkflowInstance` on a client instance will start a new workflow instance. Pass options, a workflow to run, and any inputs.

```go
wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
	InstanceID: uuid.NewString(),
}, Workflow1, "input-for-workflow")
if err != nil {
	// ...
}
```

## Removing workflow instances

`RemoveWorkflowInstance` on a client instance will remove that workflow instance including all history data from the backend. A workflow instance needs to be in the finished state before calling this, otherwise an error will be returned.

```go
err = c.RemoveWorkflowInstance(ctx, workflowInstance)
if err != nil {
	// ...
}
```

### Automatically expiring finished workflow instances

For now this is only supported for the Redis backend. When an `AutoExpiration` is passed to the backend, finished workflow instances will be automatically removed after the specified duration. This works by setting a TTL on the Redis keys for finished workflow instances. If `AutoExpiration` is set to `0` (the default), no TTL will be set.

```go
b, err := redis.NewRedisBackend(redisClient, redis.WithAutoExpiration(time.Hour * 48))
// ...
```

## Canceling workflows

Create a `Client` instance then then call `CancelWorkflow` to cancel a workflow. When a workflow is canceled, it's workflow context is canceled. Any subsequent calls to schedule activities or sub-workflows will immediately return an error, skipping their execution. Any activities already running when a workflow is canceled will still run to completion and their result will be available.

Sub-workflows will be canceled if their parent workflow is canceled.

```go
var c *client.Client
err = c.CancelWorkflowInstance(context.Background(), workflowInstance)
if err != nil {
	panic("could not cancel workflow")
}
```

### Perform any cleanup

If you need to run any activities or make calls using `workflow.Context` you need to create a new context with `workflow.NewDisconnectedContext`, since the original context is canceled at this point.

```go
func Workflow2(ctx workflow.Context, msg string) (string, error) {
	defer func() {
		if errors.Is(ctx.Err(), workflow.Canceled) {
			// Workflow was canceled. Get new context to perform any cleanup activities
			ctx := workflow.NewDisconnectedContext(ctx)

			// Execute the cleanup activity
			if err := workflow.ExecuteActivity(ctx, ActivityCleanup).Get(ctx, nil); err != nil {
				return errors.Wrap(err, "could not perform cleanup")
			}
		}
	}()

	r1, err := workflow.ExecuteActivity[int](ctx, ActivityCancel, 1, 2).Get(ctx)
	if err != nil {  // <---- Workflow is canceled while this activity is running
		return errors.Wrap(err, "could not get ActivityCancel result")
	}

	// r1 will contain the result of ActivityCancel
	// ⬇ ActivitySkip will be skipped immediately
	r2, err := workflow.ExecuteActivity(ctx, ActivitySkip, 1, 2).Get(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get ActivitySkip result")
	}

	return "some result", nil
}
```

## Running activities

From a workflow, call `workflow.ExecuteActivity` to execute an activity. The call returns a `Future[T]` you can await to get the result or any error it might return.

```go
r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12, nil, "test").Get(ctx)
if err != nil {
	panic("error getting activity 1 result")
}

log.Println(r1)
```

### Canceling activities

Canceling activities is not supported at this time.

## Timers

You can schedule timers to fire at any point in the future by calling `workflow.ScheduleTimer`. It returns a `Future` you can await to wait for the timer to fire.

```go
t := workflow.ScheduleTimer(ctx, 2*time.Second)
err := t.Get(ctx, nil)
```

### Canceling timers

There is no explicit API to cancel timers. You can cancel a timer by creating a cancelable context, and canceling that:

```go
tctx, cancel := workflow.WithCancel(ctx)
t := workflow.ScheduleTimer(tctx, 2*time.Second)

// Cancel the timer
cancel()
```

## Signals

```go
// From outside the workflow:
c.SignalWorkflow(ctx, "<instance-id>", "signal-name", "value")

func Workflow(ctx workflow.Context) error {
	// ...

	signalCh := workflow.NewSignalChannel[string](ctx, "signal-name")

	// Pause workflow until signal is received
	workflow.Select(ctx,
		workflow.Receive(signalCh, func(ctx workflow.Context, r string, ok bool) {
			logger.Debug("Received signal:", r)
		}),
	)

	// Continue execution
}
```

Signals are a way to send a message to a workflow from the outside. You can send a signal to a workflow by calling `workflow.Signal` and listen to them by creating a `SignalChannel` via `NewSignalChannel`.

### Signaling other workflows from within a workflow

```go
func Workflow(ctx workflow.Context) error {
	if _, err := workflow.SignalWorkflow(ctx, "sub-instance-id", "signal-name", "value").Get(ctx); err != nil {
		// Handle error
	}
}
```

## Executing side effects

```go
id, _ := workflow.SideEffect[string](ctx, func(ctx workflow.Context) string) {
	return uuid.NewString()
}).Get(ctx)
```

Sometimes scheduling an activity is too much overhead for a simple side effect. For those scenarios you can use `workflow.SideEffect`. You can pass a func which will be executed only once inline with its result being recorded in the history. Subsequent executions of the workflow will return the previously recorded result.

## Running sub-workflows

```go
func Workflow1(ctx workflow.Context, msg string) error {
	result, err := workflow.CreateSubWorkflowInstance[int]
		ctx, workflow.SubWorkflowInstanceOptions{}, SubWorkflow, "some input").Get(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get sub workflow result")
	}

	logger.Debug("Sub workflow result:", "result", result)

	return nil
}

func SubWorkflow(ctx workflow.Context, msg string) (int, error) {
	r1, err := workflow.ExecuteActivity[int](ctx, Activity1, 35, 12).Get(ctx)
	if err != nil {
		return "", errors.Wrap(err, "could not get activity result")
	}

	logger.Debug("A1 result:", "r1", r1)
	return r1, nil
}
```

Call `workflow.CreateSubWorkflowInstance` to start a sub-workflow. The returned `Future` will resolve once the sub-workflow has finished.

### Canceling sub-workflows

Similar to timer cancellation, you can pass a cancelable context to `CreateSubWorkflowInstance` and cancel the sub-workflow that way. Reacting to the cancellation is the same as canceling a workflow via the `Client`. See [Canceling workflows](#canceling-workflows) for more details.

## Error handling

### Custom errors

Errors returned from activities and subworkflows need to be marshalled/unmarshalled by the library so they are wrapped in a `workflow.Error`. You can access the original type via the `err.Type` field. If a stacktrace was captured, you can access it via `err.Stack()`. Example (see also `samples/errors`):

```go
func handleError(ctx workflow.Context, logger log.Logger, err error) {
	var werr *workflow.Error
	if errors.As(err, &werr) {
		switch werr.Type {
		case "CustomError": // This was a `type CustomError struct...` returned by an activity/subworkflow
			logger.Error("Custom error", "err", werr)
			return
		}

		logger.Error("Generic workflow error", "err", werr, "stack", werr.Stack())
		return
	}

	var perr *workflow.PanicError
	if errors.As(err, &perr) {
		// Activity/subworkflow ran into a panic
		logger.Error("Panic", "err", perr, "stack", perr.Stack())
		return
	}

	logger.Error("Generic error", "err", err)
}
```

### Panics

A panic in an activity will be captured by the library and made available as a `workflow.PanicError` in the calling workflow. Example:


```go
r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, "test").Get(ctx)
if err != nil {
	panic("error getting activity 1 result")
}

var perr *workflow.PanicError
if errors.As(err, &perr) {
	logger.Error("Panic", "err", perr, "stack", perr.Stack())
	return
}
```

### Retries

With the default `DefaultActivityOptions`, Activities are retried up to three times when they return an error. If you want to keep automatic retries, but want to avoid them when hitting certain error types, you can wrap an error with `workflow.NewPermanentError`:

**Workflow**:

```go
r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, "test").Get(ctx)
if err != nil {
	panic("error getting activity 1 result")
}

log.Println(r1)
```

**Activity**:

```go
func Activity1(ctx context.Context, name string) (int, error) {
	if name == "test" {
		// No need to retry in this case, the activity will aways fail with the given inputs
		return 0, workflow.NewPermanentError(errors.New("test is not a valid name"))
	}

	return http.Do("POST", "https://example.com", name)
}
```

## `ContinueAsNew`

```go
wf := func(ctx workflow.Context, run int) (int, error) {
	run = run + 1
	if run < 3 {
		return run, workflow.ContinueAsNew(ctx, run)
	}

	return run, nil
}
```

`ContinueAsNew` allows you to restart workflow execution with different inputs. The purpose is to keep the history size small enough to avoid hitting size limits, running out of memory and impacting performance. It works by returning a special `error` from your workflow that contains the new inputs.

Here the workflow is going to be restarted when `workflow.ContinueAsNew` is returned. Internally the new execution starts with a fresh history. It uses the same `InstanceID` but a different `ExecutionID`.

If a sub-workflow is restarted, the caller doesn't notice this, only once it ends without being restarted the caller will get the result and control will be passed back.

## `select`

Due its non-deterministic behavior you must not use a `select` statement in workflows. Instead you can use the provided `workflow.Select` function. It blocks until one of the provided cases is ready. Cases are evaluated in the order passed to `Select.

```go
var f1 workflow.Future[int]
var c workflow.Channel[int]

value := 42

workflow.Select(
	ctx,
	workflow.Await(f1, func (ctx workflow.Context, f Future[int]) {
		r, err := f.Get(ctx)
		// ...
	}),
	workflow.Receive(c, func (ctx workflow.Context, v int, ok bool) {
		// use v
	}),
	workflow.Send(c, &value, func (ctx workflow.Context) {
		// value has been sent to the channel
	}),
	workflow.Default(ctx, func (ctx workflow.Context) {
		// ...
	})
)
```

### Waiting for a Future

`Await` adds a case to wait for a Future to have a value

```go
var f1, f2 workflow.Future[int]

workflow.Select(
	ctx,
	workflow.Await(f1, func (ctx workflow.Context, f Future[int]) {
		r, err := f.Get(ctx)
		// ...
	}),
	workflow.Await(f2, func (ctx workflow.Context, f Future[int]) {
		r, err := f.Get(ctx)
		// ...
	}),
)
```

### Waiting to receive from a Channel

`Receive` adds a case to receive from a given channel

```go
var c workflow.Channel[int]

workflow.Select(
	ctx,
	workflow.Receive(c, func (ctx workflow.Context, v int, ok bool) {
		// ...
	}),
)
```

### Default/Non-blocking

A `Default` case is executed if no previous case is ready and selected:

```go
var f1 workflow.Future[int]

workflow.Select(
	ctx,
	workflow.Await(f1, func (ctx workflow.Context, f Future[int]) {
		r, err := f.Get(ctx, &r)
		// ...
	}),
	workflow.Default(ctx, func (ctx workflow.Context) {
		// ...
	})
)
```

## Unit testing

### Workflows

```go
func TestWorkflow(t *testing.T) {
	tester := tester.NewWorkflowTester[int](Workflow1)

	// Mock two activities
	tester.OnActivity(Activity1, mock.Anything, 35, 12).Return(47, nil)
	tester.OnActivity(Activity2, mock.Anything, mock.Anything, mock.Anything).Return(12, nil)

	// Run workflow with inputs
	tester.Execute("Hello world")

	// Workflows always run to completion, or time-out
	require.True(t, tester.WorkflowFinished())

	wr, werr := tester.WorkflowResult()
	require.Equal(t, 59, wr)
	require.Empty(t, werr)

	// Ensure any expectations set for activity or sub-workflow mocks were met
	tester.AssertExpectations(t)
}
```

go-workflows includes support for testing workflows, a simple example using mocked activities.

- Timers are automatically fired by advancing a mock workflow clock that is used for testing workflows
- You can register callbacks to fire at specific times (in mock-clock time). Callbacks can send signals, cancel workflows etc.

### Activities

```go
func Activity(ctx context.Context, a int, b int) (int, error) {
	activity.Logger(ctx).Debug("Activity is called", "a", a)

	return a + b, nil
}

func TestActivity(t *testing.T) {
	ctx := activitytester.WithActivityTestState(context.Background(), "activityID", "instanceID", nil)

	r, err := Activity(ctx, 35, 12)
	require.Equal(t, 47, r)
	require.NoError(t, err)
}
```

Activities can be tested like any other function. If you make use of the activity context, for example, to retrieve a logger, you can use `activitytester.WithActivityTestState` to provide a test activity context. If you don't specify a logger, the default logger implementation will be used.

## Logging

For logging, you can pass a type to the backend via the `WithLogger` option to set a custom logger. The type has to implement this simple interface:

```go
type Logger interface {
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Panic(msg string, fields ...interface{})

	With(fields ...interface{}) Logger
}
```

If you don't pass a logger, a very simple, unoptimized default logger is used. For production use it is strongly recommended to pass another logger.

### Workflows

For logging in workflows, you can get a logger using

```go
logger := workflow.Logger(ctx)
```

The returned `logger` implements the `Logger` interface, and already has the workflow instance set as default field.

### Activities

For logging in activities, you can get a logger using

```go
logger := activity.Logger(ctx)
```

The returned `logger` implements the `Logger` interface, and already has the id of the activity, and the workflow instance set as default field.

## Tracing

The library supports tracing via [OpenTelemetry](https://opentelemetry.io/). When you pass a `TracerProvider` when creating a backend instance, workflow execution will be traced. You can also add additional spans for both activities and workflows.

_Note: the support is considered experimental right now, if you decide to use it, please leave feedback_

### Activities

The `context.Context` passed into activities is set up with the correct current span. If you create additional spans, they'll show up under the `ActivityTaskExecution`:

```go
func Activity1(ctx context.Context, a, b int) (int, error) {
	ctx, span := otel.Tracer("activity1").Start(ctx, "Custom Activity1 span")
	defer span.End()

	// Do something
}
```

### Workflows

For workflows the usage is a bit different, the tracer needs to be aware of whether the workflow is being replayed or not:

```go
func Workflow(ctx workflow.Context) error {
	ctx, span := workflow.Tracer(ctx).Start(ctx, "Workflow1 span", trace.WithAttributes(
		// Add additional
		attribute.String("msg", "hello world"),
	))

	// Do something

	span.End()
```

## Context Propagation

In go programs it is common to use `context.Context` to pass around request-scoped data. This library supports context propagation between activities and workflows. When you create a workflow, you can pass a `ContextPropagator` to the backend to propagate context values. The interface is:

```go
type ContextPropagator interface {
	Inject(context.Context, *Metadata) error
	Extract(context.Context, *Metadata) (context.Context, error)

	InjectFromWorkflow(Context, *Metadata) error
	ExtractToWorkflow(Context, *Metadata) (Context, error)
}
```

The `context-propagation` sample shows an example of how to use this.

## Tools

### Analyzer

`/analyzer` contains a simple [golangci-lint](https://github.com/golangci/golangci-lint) based analyzer to spot common issues in workflow code.

### Diagnostics Web UI

For investigating workflows, the package includes a simple diagnostic web UI. You can serve it via:

```go
m := http.NewServeMux()
m.Handle("/diag/", http.StripPrefix("/diag", diag.NewServeMux(b)))
go http.ListenAndServe(":3000", m)
```

It provides a simple paginated list of workflow instances:

<img src="./docs/diag-list.png" width="700">

And a way to inspect the history of a workflow instance:

<img src="./docs/diag-details.png" width="700">


