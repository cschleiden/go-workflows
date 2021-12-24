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
	s      sync.Scheduler
	fn     reflect.Value
	result []byte
	err    error
}

func NewWorkflow(workflowFn reflect.Value) *workflow {
	s := sync.NewScheduler()

	return &workflow{
		s:  s,
		fn: workflowFn,
	}
}

func (w *workflow) Execute(ctx context.Context, inputs [][]byte) error {
	w.s.NewCoroutine(ctx, func(ctx context.Context) {
		args, err := inputsToArgs(ctx, w.fn, inputs)
		if err != nil {
			panic(err) // TODO: Handle error
		}

		args[0] = reflect.ValueOf(ctx)

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

	w.s.Execute(ctx)

	return nil
}

func (w *workflow) Continue(ctx context.Context) error {
	w.s.Execute(ctx)

	return nil
}

func (w *workflow) Completed() bool {
	return w.s.RunningCoroutines() == 0
}

func (w *workflow) Close(ctx context.Context) {
	// End coroutine execution to prevent goroutine leaks
	w.s.Exit(ctx)
}

func inputsToArgs(ctx context.Context, activityFn reflect.Value, inputs [][]byte) ([]reflect.Value, error) {
	activityFnT := activityFn.Type()

	numArgs := activityFnT.NumIn()
	args := make([]reflect.Value, numArgs)

	input := 0
	for i := 0; i < numArgs; i++ {
		argT := activityFnT.In(i)

		// Insert context if requested
		if i == 0 && (isContext(argT)) {
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
