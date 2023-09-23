package args

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/internal/sync"
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

func ReturnTypeMatch[TResult any](fn interface{}) error {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return errors.New("not a function")
	}

	if fnType.NumOut() < 1 {
		return errors.New("function has no return value, must return at least (error) or (result, error)")
	}

	if fnType.NumOut() > 2 {
		return errors.New("function has too many return values, must return at most (error) or (result, error)")
	}

	errorPosition := 0
	if fnType.NumOut() == 2 {
		errorPosition = 1

		t := *new(TResult)
		if fnType.Out(0) != reflect.TypeOf(t) {
			return fmt.Errorf("function must return %s, got %s", reflect.TypeOf(t), fnType.Out(0))
		}
	}

	// Check if return is error
	if fnType.Out(errorPosition) != reflect.TypeOf((*error)(nil)).Elem() {
		return fmt.Errorf("function must return error, got %s", fnType.Out(errorPosition))
	}

	return nil
}

func ParamsMatch(fn interface{}, args ...interface{}) error {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return errors.New("not a function")
	}

	requiredArguments := fnType.NumIn()
	needsContext := false
	if fnType.NumIn() > 0 {
		argT := fnType.In(0)

		if IsOwnContext(argT) || isContext(argT) {
			needsContext = true
			requiredArguments--
		}
	}

	if requiredArguments != len(args) {
		return fmt.Errorf("mismatched argument count: expected %d, got %d", requiredArguments, len(args))
	}

	targetIdx := 0
	if needsContext {
		targetIdx = 1
	}

	for _, arg := range args {
		// if target is interface{} skip
		if fnType.In(targetIdx).Kind() == reflect.Interface {
			continue
		}

		if fnType.In(targetIdx) != reflect.TypeOf(arg) {
			return fmt.Errorf("mismatched argument type: expected %s, got %s", fnType.In(targetIdx), reflect.TypeOf(arg))
		}

		targetIdx++
	}

	return nil
}

func IsOwnContext(inType reflect.Type) bool {
	contextElem := reflect.TypeOf((*sync.Context)(nil)).Elem()
	return inType != nil && inType.Implements(contextElem)
}

func isContext(inType reflect.Type) bool {
	contextElem := reflect.TypeOf((*context.Context)(nil)).Elem()
	return inType != nil && inType.Implements(contextElem)
}
