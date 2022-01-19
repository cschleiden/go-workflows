# Experiments in building durable workflows using Go

Borrows heavily from [Temporal](https://github.com/temporalio/temporal) (and since it's a fork also [Cadence](https://github.com/uber/cadence)) as well as [DTFx](https://github.com/Azure/durabletask).

## Example



### Workflow

Workflows are written in Go code. The only exception is they must not use any of Go's non-deterministic features (`select`, iteration over a `map`, etc.). Inputs and outputs for workflows and activities have to be serializable:

```go
func Workflow1(ctx workflow.Context, input string) error {
	var r1, r2 int

	if err := workflow.ExecuteActivity(ctx, Activity1, 35, 12).Get(ctx, &r1); err != nil {
		panic("error getting activity 1 result")
	}

	log.Println("A1 result:", r1)

	if err := workflow.ExecuteActivity(ctx, Activity2).Get(ctx, &r2); err != nil {
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
	w := worker.New(mb)

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
	<-c2
}
```
