---
sidebar_position: 1
---

# Creating a workflow

Workflows are written in Go code. Inputs and outputs for workflows and activities have to be serializable.

:::danger Workflows must be deterministic
Workflows must not use any of Go's non-deterministic features (`select` statement, iteration over a `map`, etc.). There are equivalent deterministic options available.
:::

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

Next: [Create an activity](./activities.md)