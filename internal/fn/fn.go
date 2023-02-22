package fn

import (
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

func ReturnTypeMatch[TResult any](fn interface{}) bool {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return false
	}

	if fnType.NumOut() == 1 {
		return true
	}

	t := *new(TResult)
	return fnType.Out(0) == reflect.TypeOf(t)
}

func ParamsMatch(fn interface{}, skip int, args ...interface{}) bool {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return false
	}

	if fnType.NumIn() != skip+len(args) {
		return false
	}

	for i, arg := range args {
		// if target is interface{} skip
		if fnType.In(skip+i).Kind() == reflect.Interface {
			continue
		}

		if fnType.In(skip+i) != reflect.TypeOf(arg) {
			return false
		}
	}

	return true
}
