package workflow

import (
	"reflect"

	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/payload"
	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/pkg/errors"
)

type Workflow interface{}

type workflow struct {
	s      sync.Scheduler
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
	w.s.NewCoroutine(ctx, func(ctx sync.Context) {
		args, err := inputsToArgs(w.fn, inputs)
		if err != nil {
			panic(err) // TODO: Handle error
		}

		args[0] = reflect.ValueOf(ctx)

		r := w.fn.Call(args)

		if len(r) < 1 || len(r) > 2 {
			//return errors.New("workflow has to return either (error) or (result, error)") // TODO: error handling
			panic("workflow has to return either (error) or (result, error)")
		}

		var result payload.Payload

		if len(r) > 1 {
			var err error
			result, err = converter.DefaultConverter.To(r[0].Interface())
			if err != nil {
				// return nil, errors.Wrap(err, "could not convert workflow result")
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

	return w.s.Execute(ctx)
}

func (w *workflow) Continue(ctx sync.Context) error {
	return w.s.Execute(ctx)
}

func (w *workflow) Completed() bool {
	return w.s.RunningCoroutines() == 0
}

func (w *workflow) Result() payload.Payload {
	return w.result
}

func (w *workflow) Error() error {
	return w.err
}

func (w *workflow) Close(ctx sync.Context) {
	// End coroutine execution to prevent goroutine leaks
	w.s.Exit(ctx)
}

// TODO: Extract to converter package
func inputsToArgs(activityFn reflect.Value, inputs []payload.Payload) ([]reflect.Value, error) {
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
	contextElem := reflect.TypeOf((*sync.Context)(nil)).Elem()
	return inType != nil && inType.Implements(contextElem)
}
