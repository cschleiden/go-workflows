package workflow

import (
	"context"
	"fmt"
	"testing"

	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/tasks"
	"github.com/cschleiden/go-dt/pkg/history"
)

func Test_ExecuteWorkflow(t *testing.T) {
	r := NewRegistry()

	r.RegisterWorkflow("w1", Workflow1)
	r.RegisterActivity("a1", Activity1)
	r.RegisterActivity("a2", Activity2)

	e := NewExecutor(r)

	e.ExecuteWorkflowTask(context.Background(), tasks.WorkflowTask{
		WorkflowInstance: core.NewWorkflowInstance("instanceID", "executionID"),
		History: []history.HistoryEvent{
			history.NewHistoryEvent(
				history.HistoryEventType_WorkflowExecutionStarted,
				-1,
				&history.ExecutionStartedAttributes{
					Name:    "",
					Version: "",
					Inputs:  [][]byte{},
				},
			),
		},
	})
}

func Workflow1(ctx Context) error {
	fmt.Println("Entering Workflow1")
	// fmt.Println("\tIsReplaying:", ctx.IsReplaying())

	// a1, err := workflow.ExecuteActivity(ctx, Activity1)
	// if err != nil {
	// 	panic("error executing activity 1")
	// }

	// r1, err := a1.Get()
	// if err != nil {
	// 	panic("error getting activity 1 result")
	// }
	// fmt.Println("R1 result:", r1)

	// a2, err := workflow.ExecuteActivity(ctx, Activity2)
	// if err != nil {
	// 	panic("error executing activity 1")
	// }

	// r2, err := a2.Get()
	// if err != nil {
	// 	panic("error getting activity 1 result")
	// }
	// fmt.Println("R2 result:", r2)

	fmt.Println("Leaving Workflow1")

	return nil
}

func Activity1(ctx Context) (int, error) {
	fmt.Println("Entering Activity1")

	return 42, nil
}

func Activity2(ctx Context) (string, error) {
	fmt.Println("Entering Activity2")

	return "hello", nil
}
