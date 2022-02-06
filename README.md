# Experiments in building durable workflows using Go

Borrows heavily from [Temporal](https://github.com/temporalio/temporal) (and since it's a fork also [Cadence](https://github.com/uber/cadence)) as well as [DTFx](https://github.com/Azure/durabletask).

## Simple example

### Workflow

Workflows are written in Go code. The only exception is they must not use any of Go's non-deterministic features (`select`, iteration over a `map`, etc.). Inputs and outputs for workflows and activities have to be serializable:

```go
func Workflow1(ctx workflow.Context, input string) error {
	var r1, r2 int

	if err := workflow.ExecuteActivity(ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx, &r1); err != nil {
		panic("error getting activity 1 result")
	}

	log.Println("A1 result:", r1)

	if err := workflow.ExecuteActivity(ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx, &r2); err != nil {
		panic("error getting activity 1 result")
	}

	log.Println("A2 result:", r2)

	return nil
}
```

### Activities

Activities can have side-effects and don't have to be deterministic. They will be executed only once and the result is persisted:

```go
func Activity1(ctx context.Context, a, b int) (int, error) {
	return a + b, nil
}

func Activity2(ctx context.Context) (int, error) {
	return 12, nil
}

```

### Worker

The worker is responsible for executing `Workflows` and `Activities`, both need to be registered with it.

```go
func runWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb, nil)

	r.RegisterWorkflow(Workflow1)

	w.RegisterActivity(Activity1)
	w.RegisterActivity(Activity2)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}
```

### Backend

The backend is responsible for persisting the workflow events. Currently there is an in-memory backend implementation for testing, one using [SQLite](http://sqlite.org), and one for MySql.

```go
b := sqlite.NewSqliteBackend("simple.sqlite")
```

### Putting it all together

We can start workflows from the same process the worker runs in -- or they can be separate. Here we use the SQLite backend, spawn a single worker (which then executes both `Workflows` and `Activities`), and then start a single instance of our workflow

```go
func main() {
	ctx := context.Background()

	b := sqlite.NewSqliteBackend("simple.sqlite")

	go runWorker(ctx, b)

	c := client.NewClient(b)

	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "input-for-workflow")
	if err != nil {
		panic("could not start workflow")
	}

	c2 := make(chan os.Signal, 1)
signal.Notify(c2, os.Interrupt)
	signal.Notify(c2, os.Interrupt)
	<-c2
}
```

## Architecture (WIP)

The high-level architecture follows the same model as Azure's [DurableTask](https://github.com/Azure/durabletask) library. The "persistence store" or "providers" mentioned there are implementations of `backend.Backend` for `go-dt`. There is no intermediate server, clients which create new workflow instances and retrieve their results, as well as worker processes (which could also be the same as clients), talk directly to the backend. For the two implemented backends so far, that also means directly to the database.

![](./docs/high-level-arch.drawio.png)

### Execution model

While there are implementations for other lanuages in the context of Azure Durable Functions, the general purpose version of Durable Task was only implemented for C#.

The execution model for `go-dt` follows closely the one created for Uber's [Cadence](https://cadenceworkflow.io) and which was then forked by the original creators for [Temporal.io](https://temporal.io).

TODO: describe in more detail here.

### Supported backends

For all backends, for now the initial schema is applied upon first usage. In the future this might move to something more powerful to migrate between versions, but in this early stage, there is no upgrade.

#### Sqlite

The Sqlite backend implementation supports two different modes, in-memory and on-disk.

 * In-memory:
	```go
	b := sqlite.NewInMemoryBackend()
	```
 * On-disk:
	```go
	b := sqlite.NewSqliteBackend("simple.sqlite")
	```

#### MySql

```go
b := mysql.NewMysqlBackend("localhost", 3306, "root", "SqlPassw0rd", "simple")
```

## Scenarios

## Registering workflows

Workflows need to accept `workflow.Context` as their first parameter, and any number of inputs parameters afterwards. Parameters need to be serializable (e.g., no `chan`s etc.). Workflows need to return an `error` and optionally one additional result, which again needs to be serializable.

```go
func Workflow1(ctx workflow.Context) error {
}
```

Workflows needs to be registered with the worker before they can be started:

```go
var b backend.Backend
w := worker.New(b)

w.RegisterWorkflow(Workflow1)
```

## Registering activities

Similar to workflows, activities need to be registered with the worker before they can be started. They also need to accept `context.Context` as their first parameter, and any number of inputs parameters afterwards. Parameters need to be serializable (e.g., no `chan`s etc.). Activities need to return an `error` and optionally one additional result, which again needs to be serializable.

Activites can be registered as plain `func`s or as methods on a struct. The latter is useful if you want to provide some shared state to activities, for example, a database connection.

```go
func Activity1(ctx context.Context, a, b int) (int, error) {
	return a + b, nil
}
```

```go
var b backend.Backend
w := worker.New(b)

w.RegisterActivity(Activity1)
```

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
```

```go
var b backend.Backend
w := worker.New(b)

w.RegisterActivity(&act{SharedState: 12})
```

to call activities registered on a struct from a workflow:

```go
// ...
var a *act

if err := workflow.ExecuteActivity(ctx, workflow.DefaultActivityOptions, a.Activity1, 35, 12).Get(ctx, &r1); err != nil {
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
```

## Running activities

From a workflow, call `workflow.ExecuteActivity` to execute an activity. The call returns a `Future` you can await to get the result or any error it might return.

```go
var r1 int
err := workflow.ExecuteActivity(ctx, Activity1, 35, 12, nil, "test").Get(ctx, &r1)
if err != nil {
	panic("error getting activity 1 result")
}

log.Println(r1)
```

## Running sub-workflows

Call `workflow.CreateSubWorkflowInstance` to start a sub-workflow.

```go
func Workflow1(ctx workflow.Context, msg string) error {
	var wr int
	if err := workflow.CreateSubWorkflowInstance(ctx, workflow.SubWorkflowInstanceOptions{}, Workflow2, "some input").Get(ctx, &wr); err != nil {
		return errors.Wrap(err, "could not get sub workflow result")
	}

	log.Println("Sub workflow result:", wr)
	return nil
}

func Workflow2(ctx workflow.Context, msg string) (int, error) {
	var r1 int

	if err := workflow.ExecuteActivity(ctx, Activity1, 35, 12).Get(ctx, &r1); err != nil {
		return "", errors.Wrap(err, "could not get activity result")
	}

	log.Println("A1 result:", r1)

	return r1, nil
}
```

## Canceling workflows

Create a `Client` instance then then call `CancelWorkflow` to cancel a workflow. When a workflow is canceled, it's workflow context is canceled. Any subsequent calls to schedule activities or sub-workflows will immediately return an error, skipping their execution. Activities or sub-workflows already running when a workflow is canceled will still run to completion and their result will be available.

Sub-workflows will be canceled if their parent workflow is canceled.

```go
var c client.Client
err = c.CancelWorkflowInstance(context.Background(), workflowInstance)
if err != nil {
	panic("could not cancel workflow")
}
```

### Perform any cleanup

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

	var r1 int
	if err := workflow.ExecuteActivity(ctx, ActivityCancel, 1, 2).Get(ctx, &r1); err != nil {  // <---- Workflow is canceled while this activity is running
		return errors.Wrap(err, "could not get ActivityCancel result")
	}

	// r1 will contain the result of ActivityCancel
	// ⬇ ActivitySkip will be skipped immediately
	var r2 int
	if err := workflow.ExecuteActivity(ctx, ActivitySkip, 1, 2).Get(ctx, &r2); err != nil {
		return errors.Wrap(err, "could not get ActivitySkip result")
	}

	return "some result", nil
}
```