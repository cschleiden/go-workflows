package fn

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

func Name(i interface{}) string {
	// Adapted from https://stackoverflow.com/a/7053871
	fnName := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()

	s := strings.Split(fnName, ".")
	fnName = s[len(s)-1]

	return strings.TrimSuffix(fnName, "-fm")
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

func ParamsMatch(fn interface{}, skip int, args ...interface{}) error {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return errors.New("not a function")
	}

	if fnType.NumIn() != skip+len(args) {
		return fmt.Errorf("mismatched argument count: expected %d, got %d", fnType.NumIn()-skip, len(args))
	}

	for i, arg := range args {
		// if target is interface{} skip
		if fnType.In(skip+i).Kind() == reflect.Interface {
			continue
		}

		if fnType.In(skip+i) != reflect.TypeOf(arg) {
			return fmt.Errorf("mismatched argument type: expected %s, got %s", fnType.In(skip+i), reflect.TypeOf(arg))
		}
	}

	return nil
}
