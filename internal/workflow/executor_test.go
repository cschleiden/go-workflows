package workflow

// import (
// 	"context"
// 	"fmt"
// 	"testing"
// )

// func Test_ExecuteWorkflow(t *testing.T) {
// 	e := &executorImpl{}

// 	e.ExecuteWorkflow(context.Background(), Workflow1)
// }

// func Workflow1(ctx Context) error {
// 	fmt.Println("Entering Workflow1")
// 	fmt.Println("\tIsReplaying:", ctx.IsReplaying())

// 	a1, err := ExecuteActivity(ctx, Activity1)
// 	if err != nil {
// 		panic("error executing activity 1")
// 	}

// 	r1, err := a1.Get()
// 	if err != nil {
// 		panic("error getting activity 1 result")
// 	}
// 	fmt.Println("R1 result:", r1)

// 	a2, err := ExecuteActivity(ctx, Activity2)
// 	if err != nil {
// 		panic("error executing activity 1")
// 	}

// 	r2, err := a2.Get()
// 	if err != nil {
// 		panic("error getting activity 1 result")
// 	}
// 	fmt.Println("R2 result:", r2)

// 	fmt.Println("Leaving Workflow1")

// 	return nil
// }

// func Activity1(ctx Context) (int, error) {
// 	fmt.Println("Entering Activity1")

// 	return 42, nil
// }

// func Activity2(ctx Context) (string, error) {
// 	fmt.Println("Entering Activity2")

// 	return "hello", nil
// }
