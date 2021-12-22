package workflow

import (
	"context"
	"reflect"

	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/cschleiden/go-dt/pkg/converter"
	"github.com/pkg/errors"
)

type Workflow interface{}

type workflow struct {
	context *contextImpl
	cr      sync.Coroutine
	fn      reflect.Value
	result  []byte
	err     error
}

func NewWorkflow(workflowFn reflect.Value) *workflow {
	c := sync.NewCoroutine()

	return &workflow{
		context: newWorkflowContext(c),
		cr:      c,
		fn:      workflowFn,
	}
}

func (w *workflow) Context() *contextImpl {
	return w.context
}

func (w *workflow) Execute(ctx context.Context, inputs [][]byte) error {
	w.cr.Run(ctx, func(ctx context.Context) {
		// TODO: Support inputs
		args, err := inputsToArgs(ctx, w.fn, inputs)
		if err != nil {
			panic(err) // TODO: Handle error
		}

		args[0] = reflect.ValueOf(w.context)

		r := w.fn.Call(args)

		if len(r) < 1 || len(r) > 2 {
			//return errors.New("workflow has to return either (error) or (result, error)") // TODO: error handling
			panic("workflow has to return either (error) or (result, error)")
		}

		var result []byte

		if len(r) > 1 {
			var err error
			result, err = converter.DefaultConverter.To(r[0].Interface())
			if err != nil {
				// return nil, errors.Wrap(err, "could not convert activity result")
				// TODO: return error from workflow
			}
		}

		errResult := r[len(r)-1]
		if errResult.IsNil() {
			w.result = result
			return
		}

		errInterface, ok := errResult.Interface().(error)
		if !ok {
			// return nil, fmt.Errorf("activity error result does not satisfy error interface (%T): %v", errResult, errResult)
			// TODO: Handle workflow infrastructure error
		}

		w.err = errInterface
	})

	w.cr.WaitUntilBlocked()

	return nil
}

func (w *workflow) Continue(ctx context.Context) error {
	w.cr.Execute()

	return nil
}

func (w *workflow) Completed() bool {
	return w.cr.Finished()
}

func (w *workflow) Close() {
	// End coroutine execution to prevent goroutine leaks
	w.cr.Exit()
}

func inputsToArgs(ctx context.Context, activityFn reflect.Value, inputs [][]byte) ([]reflect.Value, error) {
	activityFnT := activityFn.Type()

	numArgs := activityFnT.NumIn()
	args := make([]reflect.Value, numArgs)

	input := 0
	for i := 0; i < numArgs; i++ {
		argT := activityFnT.In(i)

		// Insert context if requested
		if i == 0 && (isContext(argT) || isWorkflowContext(argT)) {
			continue
		}

		arg := reflect.New(argT).Interface()
		err := converter.DefaultConverter.From(inputs[input], arg)
		if err != nil {
			return nil, errors.Wrap(err, "could not convert activity input")
		}

		args[i] = reflect.ValueOf(arg).Elem()

		input++
	}

	return args, nil
}

func isContext(inType reflect.Type) bool {
	contextElem := reflect.TypeOf((*context.Context)(nil)).Elem()
	return inType != nil && inType.Implements(contextElem)
}

func isWorkflowContext(inType reflect.Type) bool {
	contextElem := reflect.TypeOf((*Context)(nil)).Elem()
	return inType != nil && inType.Implements(contextElem)
}
