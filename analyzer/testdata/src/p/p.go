// nolint
package p

import (
	"context"
	"fmt"
	"time"

	workflow "github.com/cschleiden/go-workflows/workflow"

	"sync"
)

var foo int = 42

func wfSimple(ctx workflow.Context) error {
	fmt.Println(foo)

	return nil
}

func wfWithResult(ctx workflow.Context) (string, error) {
	return "", nil
}

func wfWithTooManyResults(ctx workflow.Context) (int, string, error) { // want "workflow `wfWithTooManyResults` returns more than two values"
	return 42, "", nil
}

func wfWrongOrder(ctx workflow.Context) (error, string) { // want "workflow `wfWrongOrder` doesn't return `error` as last return value"
	return nil, ""
}

func wfWithoutReturn(ctx workflow.Context) { // want "workflow `wfWithoutReturn` doesn't return anything. needs to return at least `error`"
}

func wfIteratingOverMap(ctx workflow.Context) error {
	x := make(map[string]string)

	fmt.Println("log")

	for _, v := range x { // want "iterating over a `map` is not deterministic and not allowed in workflows"
		if v == "a" {
			return nil
		}
	}

	return nil
}

func wfUsingGoRoutine(ctx workflow.Context) error {
	go func() { // want "use `workflow.Go` instead of `go` in workflows"
		fmt.Println("hello")
	}()

	return nil
}

func wfUsingSelect(ctx workflow.Context) error {
	select { // want "`select` statements are not allowed in workflows, use `workflow.Select` instead"
	}

	return nil
}

func wfChanRange(ctx workflow.Context) error {
	for range make(chan int, 0) { // want "using native channels is not allowed in workflows, use `workflow.Channel` instead"
		if ctx == nil {
			v := make(chan int, 0) // want "using native channels is not allowed in workflows, use `workflow.Channel` instead"
			<-v

			for range make(chan int, 0) { // want "using native channels is not allowed in workflows, use `workflow.Channel` instead"
			}
		}
	}

	return nil
}

func wfVarUsage(ctx workflow.Context) error {
	var wg sync.WaitGroup // want "`sync.WaitGroup` is not allowed in workflows"
	wg.Wait()

	wg2 := &sync.WaitGroup{} // want "`sync.WaitGroup` is not allowed in workflows"
	wg2.Wait()

	return nil
}

func wfChanNestedFunc(ctx workflow.Context) error {
	x := func() {
		select {} // want "`select` statements are not allowed in workflows, use `workflow.Select` instead"
	}

	x()

	return nil
}

func wfFunctionUsage(ctx workflow.Context) error {
	time.Sleep(10 * time.Second) // want "`time.Sleep` is not allowed in workflows, use `workflow.Sleep` instead"
	fmt.Println(time.Now())      // want "`time.Now` is not allowed in workflows, use `workflow.Now` instead"

	return nil
}

func activity(ctx context.Context) error {
	go fmt.Println("test")

	select {}

	return nil
}

type SomeField struct {
	f func() error
}

var sf SomeField = SomeField{
	f: func() error {
		go fmt.Println("24")

		c := make(chan int)
		select {
		case <-c:
		}

		for x := range c {
			fmt.Println(x)
		}

		return nil
	},
}
