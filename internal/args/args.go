package args

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ticctech/go-workflows/internal/converter"
	"github.com/ticctech/go-workflows/internal/payload"
	"github.com/ticctech/go-workflows/internal/sync"
)

func ArgsToInputs(c converter.Converter, args ...interface{}) ([]payload.Payload, error) {
	inputs := make([]payload.Payload, 0)

	for _, arg := range args {
		input, err := c.To(arg)
		if err != nil {
			return nil, fmt.Errorf("converting args to inputs: %w", err)
		}
		inputs = append(inputs, input)
	}

	return inputs, nil
}

func InputsToArgs(c converter.Converter, fn reflect.Value, inputs []payload.Payload) ([]reflect.Value, bool, error) {
	addContext := false

	activityFnT := fn.Type()
	numArgs := activityFnT.NumIn()

	args := make([]reflect.Value, numArgs)

	input := 0
	for i := 0; i < numArgs; i++ {
		argT := activityFnT.In(i)

		// Insert context if requested
		if i == 0 && (IsOwnContext(argT) || isContext(argT)) {
			addContext = true
			continue
		}

		if input < len(inputs) {
			arg := reflect.New(argT).Interface()
			err := c.From(inputs[input], arg)
			if err != nil {
				return nil, false, fmt.Errorf("converting inputs: %w", err)
			}

			args[i] = reflect.ValueOf(arg).Elem()
		}

		input++
	}

	if addContext {
		if (numArgs - 1) != len(inputs) {
			return nil, false, fmt.Errorf("mismatched argument count: expected %d, got %d", numArgs-1, len(inputs))
		}
	} else if numArgs != len(inputs) {
		return nil, false, fmt.Errorf("mismatched argument count: expected %d, got %d", numArgs, len(inputs))
	}

	return args, addContext, nil
}

func IsOwnContext(inType reflect.Type) bool {
	contextElem := reflect.TypeOf((*sync.Context)(nil)).Elem()
	return inType != nil && inType.Implements(contextElem)
}

func isContext(inType reflect.Type) bool {
	contextElem := reflect.TypeOf((*context.Context)(nil)).Elem()
	return inType != nil && inType.Implements(contextElem)
}
