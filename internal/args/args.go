package args

import (
	"context"
	"reflect"

	"github.com/cschleiden/go-dt/internal/converter"
	"github.com/cschleiden/go-dt/internal/payload"
	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/pkg/errors"
)

func ArgsToInputs(c converter.Converter, args ...interface{}) ([]payload.Payload, error) {
	inputs := make([]payload.Payload, 0)

	for _, arg := range args {
		input, err := c.To(arg)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert activity input")
		}
		inputs = append(inputs, input)
	}

	return inputs, nil
}

func InputsToArgs(c converter.Converter, fn reflect.Value, inputs []payload.Payload) ([]reflect.Value, error) {
	activityFnT := fn.Type()

	numArgs := activityFnT.NumIn()
	args := make([]reflect.Value, numArgs)

	input := 0
	for i := 0; i < numArgs; i++ {
		argT := activityFnT.In(i)

		// Insert context if requested
		if i == 0 && (isOwnContext(argT) || isContext(argT)) {
			continue
		}

		arg := reflect.New(argT).Interface()
		err := c.From(inputs[input], arg)
		if err != nil {
			return nil, errors.Wrap(err, "could not convert inputs")
		}

		args[i] = reflect.ValueOf(arg).Elem()

		input++
	}

	return args, nil
}

func isOwnContext(inType reflect.Type) bool {
	contextElem := reflect.TypeOf((*sync.Context)(nil)).Elem()
	return inType != nil && inType.Implements(contextElem)
}

func isContext(inType reflect.Type) bool {
	contextElem := reflect.TypeOf((*context.Context)(nil)).Elem()
	return inType != nil && inType.Implements(contextElem)
}
