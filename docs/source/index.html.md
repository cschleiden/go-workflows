---
title: go-workflows

toc_footers:
  - <a href='https://github.com/cschleiden/go-workflows'>GitHub</a>

includes:
  - guide
  - samples
  - backends
  - faq

search: true

code_clipboard: true

meta:
  - name: description
    content: Documentation for go-workflows
---

# go-workflows

go-workflows is an embedded engine for orchestrating long running processes or "workflows" written in Go.

It borrows heavily from [Temporal](https://github.com/temporalio/temporal) (and since it's a fork also [Cadence](https://github.com/uber/cadence)) as well as Azure's [Durable Task Framework (DTFx)](https://github.com/Azure/durabletask). Workflows are written in plain Go.

go-workflows support pluggable backends with official implementations for Sqlite, MySql, and Redis.

See also the following blog posts:

* [https://cschleiden.dev/blog/2022-02-13-go-workflows-part1/](https://cschleiden.dev/blog/2022-02-13-go-workflows-part1/)
* [https://cschleiden.dev/blog/2022-05-02-go-workflows-part2/](https://cschleiden.dev/blog/2022-05-02-go-workflows-part2/)

# Quickstart

A short walkthrough of the most important concepts:

## Workflow

```go
func Workflow1(ctx workflow.Context, input string) error {
	r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)
	if err != nil {
		panic("error getting activity 1 result")
	}

	log.Println("A1 result:", r1)

	r2, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx)
	if err != nil {
		panic("error getting activity 1 result")
	}

	log.Println("A2 result:", r2)

	return nil
}
```

Let's first write a simple workflows. Our workflow executes two _activities_ in sequence waiting for each result. Both workflows and activities are written in plain Go. Workflows can be long-running and have to be deterministic so that they can be interrupted and resumed. Activities are functions that can have side-effects and don't have to be deterministic.

Both workflows and activities support arbitrary inputs and outputs as long as those are serializable.

Workflows have to take a `workflow.Context` as their first argument.

## Activities

```go
func Activity1(ctx context.Context, a, b int) (int, error) {
	return a + b, nil
}

func Activity2(ctx context.Context) (int, error) {
	return 12, nil
}
```

Activities receive a plain `context.Context` as their first argument. Activities are automatically retried by default, so it's good practice to make them idempotent.

## Worker

```go
func runWorker(ctx context.Context, mb backend.Backend) {
	w := worker.New(mb, nil)

	w.RegisterWorkflow(Workflow1)

	w.RegisterActivity(Activity1)
	w.RegisterActivity(Activity2)

	if err := w.Start(ctx); err != nil {
		panic("could not start worker")
	}
}
```

Next, we'll have to start a _worker_. Workers are responsible for executing workflows and activities and therefore we need to register both with the worker.

Backends support multiple worker processes, so you can scale out horizontially.

## Backend

```go
b := sqlite.NewSqliteBackend("simple.sqlite")
```

The backend is responsible for persisting the workflow events. Currently there is an in-memory backend implementation for testing, one using [SQLite](http://sqlite.org), one using MySql, and one using Redis. See [backends](#backends) for more information.

## Putting it all together

```go
func main() {
	ctx := context.Background()

	b := sqlite.NewSqliteBackend("simple.sqlite")

	go runWorker(ctx, b)

	c := client.New(b)

	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, Workflow1, "input-for-workflow")
	if err != nil {
		panic("could not start workflow")
	}

	c2 := make(chan os.Signal, 1)
	signal.Notify(c2, os.Interrupt)
	<-c2
}
```

To finish the example, we create the backend, start a worker in a separate go-routine. We then create a `Client` instance which we then  use to create a new _workflow instance_. A workflow instance is just one running instance of a previously registered workflow.

With the exception of the in-memory backend, we do not have to start the workflow from the same process the worker runs in, we could create the client from another process and create/wait for/cancel/... workflow instances from there.
