package executor

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/contextvalue"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
)

type workflow struct {
	s      *sync.Scheduler
	fn     reflect.Value
	result payload.Payload
	err    error
}

func NewWorkflow(workflowFn reflect.Value) *workflow {
	s := sync.NewScheduler()

	return &workflow{
		s:  s,
		fn: workflowFn,
	}
}

func (w *workflow) Execute(ctx sync.Context, inputs []payload.Payload) error {
	w.s.NewCoroutine(ctx, func(ctx sync.Context) error {
		converter := contextvalue.Converter(ctx)
		args, addContext, err := args.InputsToArgs(converter, w.fn, inputs)
		if err != nil {
			return fmt.Errorf("converting workflow inputs: %w", err)
		}

		if !addContext {
			return errors.New("workflow must accept context as first argument")
		}

		args[0] = reflect.ValueOf(ctx)

		// Handle panics in workflows
		defer func() {
			if r := recover(); r != nil {
				w.err = workflowerrors.NewPanicError(fmt.Sprintf("panic in workflow: %v", r))
			}
		}()

		// Call workflow function
		r := w.fn.Call(args)

		// Process result
		if len(r) < 1 || len(r) > 2 {
			return errors.New("workflow has to return either (error) or (result, error)")
		}

		var result payload.Payload

		if len(r) > 1 {
			var err error
			result, err = converter.To(r[0].Interface())
			if err != nil {
				return fmt.Errorf("converting workflow result: %w", err)
			}
		} else {
			result, err = converter.To(nil)
			if err != nil {
				return fmt.Errorf("converting workflow result: %w", err)
			}
		}

		w.result = result

		errResult := r[len(r)-1]
		if !errResult.IsNil() {
			errInterface, ok := errResult.Interface().(error)
			if !ok {
				return fmt.Errorf("workflow error result does not satisfy error interface (%T): %v", errResult, errResult)
			}

			w.err = errInterface
		}

		return nil
	})

	return w.s.Execute()
}

func (w *workflow) Continue() error {
	return w.s.Execute()
}

func (w *workflow) Completed() bool {
	return w.s.RunningCoroutines() == 0
}

// Result returns the return value of a finished workflow as a payload
func (w *workflow) Result() payload.Payload {
	return w.result
}

// Error returns the error of a finished workflow, can be nil
func (w *workflow) Error() error {
	return w.err
}

func (w *workflow) Close() {
	// End coroutine execution to prevent goroutine leaks
	w.s.Exit()
}
