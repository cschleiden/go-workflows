package workflow

import (
	"reflect"

	"github.com/cschleiden/go-dt/internal/args"
	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/payload"
	"github.com/cschleiden/go-dt/internal/sync"
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
		args, err := args.InputsToArgs(converter.DefaultConverter, w.fn, inputs)
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
