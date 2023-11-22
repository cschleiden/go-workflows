---
title: go-workflows

toc_footers:
  - <a href='https://github.com/cschleiden/go-workflows'>GitHub</a>

includes:
  - errors
  - samples
  - backends

search: true

code_clipboard: true

meta:
  - name: description
    content: Documentation for go-workflows
---

# go-workflows

go-workflows is an embedded or two tier engine for orchestrating long running processes or "workflows" written in Go.

It borrows heavily from [Temporal](https://github.com/temporalio/temporal) (and since it's a fork also [Cadence](https://github.com/uber/cadence)) as well as Azure's [Durable Task Framework (DTFx)](https://github.com/Azure/durabletask).

See also:
- https://cschleiden.dev/blog/2022-02-13-go-workflows-part1/
- https://cschleiden.dev/blog/2022-05-02-go-workflows-part2/

# Quickstart

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

Workflows are written in plain Go code. The only exception is they must not use any of Go's non-deterministic features. For example:

- `select`
- iteration over a `map`
- goroutines

Inputs and outputs for workflows and activities have to be serializable:

## Activities

```go
func Activity1(ctx context.Context, a, b int) (int, error) {
	return a + b, nil
}

func Activity2(ctx context.Context) (int, error) {
	return 12, nil
}
```

Activities can have side-effects and don't have to be deterministic. They will be executed only once and the result is persisted:


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

The worker is responsible for executing `Workflows` and `Activities`, both need to be registered with it.

## Backend

```go
b := sqlite.NewSqliteBackend("simple.sqlite")
```

The backend is responsible for persisting the workflow events. Currently there is an in-memory backend implementation for testing, one using [SQLite](http://sqlite.org), one using MySql, and one using Redis. See [backends](#backends) for more information.

## Putting it all together

We can start workflows from the same process the worker runs in -- or they can be separate. Here we use the SQLite backend, spawn a single worker (which then executes both `Workflows` and `Activities`), and then start a single instance of our workflow

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

# Guides

## Writing workflows

- avoid

## Writing activities

- idempotency

# Reference

## Create

Test

This endpoint retrieves a specific kitten.

<aside class="warning">Inside HTML code blocks like this one, you can't use Markdown, so use <code>&lt;code&gt;</code> blocks to denote code.</aside>

### HTTP Request

`GET http://example.com/kittens/<ID>`

### URL Parameters

Parameter | Description
--------- | -----------
ID | The ID of the kitten to retrieve

## Delete a Specific Kitten

```ruby
require 'kittn'

api = Kittn::APIClient.authorize!('meowmeowmeow')
api.kittens.delete(2)
```

```python
import kittn

api = kittn.authorize('meowmeowmeow')
api.kittens.delete(2)
```

```shell
curl "http://example.com/api/kittens/2" \
  -X DELETE \
  -H "Authorization: meowmeowmeow"
```

```javascript
const kittn = require('kittn');

let api = kittn.authorize('meowmeowmeow');
let max = api.kittens.delete(2);
```

> The above command returns JSON structured like this:

```json
{
  "id": 2,
  "deleted" : ":("
}
```

This endpoint deletes a specific kitten.

### HTTP Request

`DELETE http://example.com/kittens/<ID>`

### URL Parameters

Parameter | Description
--------- | -----------
ID | The ID of the kitten to delete
