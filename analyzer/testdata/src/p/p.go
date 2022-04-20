package p

// Work around module issues. The analyzer just looks for `workflow.Context` currently
import (
	workflow "context"
	"fmt"
)

func wf(ctx workflow.Context) error {
	return nil
}

func wfWithResult(ctx workflow.Context) (string, error) {
	return "", nil
}

func wfWithTooManyResults(ctx workflow.Context) (int, string, error) { // want "workflow \"wfWithTooManyResults\" returns more than two values"
	return 42, "", nil
}

func wfWrongOrder(ctx workflow.Context) (error, string) { // want "workflow \"wfWrongOrder\" doesn't return `error` as last return value"
	return nil, ""
}

func wfWithoutReturn(ctx workflow.Context) { // want "workflow \"wfWithoutReturn\" doesn't return anything. needs to return at least `error`"
}

func wfIteratingOverMap(ctx workflow.Context) error {
	x := make(map[string]string)

	fmt.Println("log")

	for _, v := range x { // want "iterating over a map is not deterministic and not allowed in workflows"
		if v == "a" {
			return nil
		}
	}

	return nil
}

func wfUsingGoRoutine(ctx workflow.Context) error {
	go func() { // want "use workflow.Go instead of `go` in workflows"
		fmt.Println("hello")
	}()

	return nil
}
