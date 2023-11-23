# FAQ

## How are releases versioned?

For now this library is in a pre-release state. There are no guarantees given regarding breaking changes between (pre)-releases.

## Workflow versioning

For now, I've intentionally left out versioning. Cadence, Temporal, and DTFx all support the concept of versions for workflows as well as activities. This is mostly required when you make changes to workflows and need to keep backwards compatibility with workflows that are being executed at the time of the upgrade.

**Example**: when you change a workflow from:

```go
func Workflow1(ctx workflow.Context) {
	r1, _ := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)
	log.Println("A1 result:", r1)

	r2, _ := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx)
	log.Println("A2 result:", r2)
}
```

to:

```go
func Workflow1(ctx workflow.Context) {
	var r1 int
	r1, _ := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)
	log.Println("A1 result:", r1)

	r3, _ := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity3).Get(ctx)
	log.Println("A3 result:", r3)

	r2, _ := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx)
	log.Println("A2 result:", r2)
}
```

and you replay a workflow history that contains:

1. `ActivitySchedule` - `Activity1`
1. `ActivityCompleted` - `Activity1`
1. `ActivitySchedule` - `Activity2`
1. `ActivityCompleted` - `Activity2`

the workflow will encounter an attempt to execute `Activity3` in-between event 2 and 3, for which there is no matching event. This is a non-recoverable error. The usual approach to solve is to version the workflows and every time you make a change to a workflow, you have to check that logic. For this example this could look like:


```go
func Workflow1(ctx workflow.Context) {
	r1, _ := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity1, 35, 12).Get(ctx)
	log.Println("A1 result:", r1)

	if workflow.Version(ctx) >= 2 {
		r3, _ := workflow.ExecuteActivity(ctx, workflow.DefaultActivityOptions, Activity3).Get(ctx)
		log.Println("A3 result:", r3)
	}

	r2, _ := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, Activity2).Get(ctx)
	log.Println("A2 result:", r2)
}
```

and only if a workflow instance was created with a version of `>= 2` will `Activity3` be executed. Older workflows are persisted with a version `< 2` and will not execute `Activity3`.

This kind of check is understandable for simple changes, but it becomes hard and a source of bugs for more complicated workflows. Therefore for now versioning is not supported and the guidance is to rely on **side-by-side** deployments. See also Azure's [Durable Functions](https://docs.microsoft.com/en-us/azure/azure-functions/durable/durable-functions-versioning) documentation for the same topic.
