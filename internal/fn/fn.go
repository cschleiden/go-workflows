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

func CheckReturn[TResult any](fn interface{}) error {
	fv := reflect.ValueOf(fn)
	ft := fv.Type()

	if ft.Kind() != reflect.Func {
		return errors.New("not a function")
	}

	if ft.NumOut() == 0 {
		return errors.New("no return value")
	}

	firstResult := ft.Out(0)

	zr := *new(TResult)
	rt := reflect.TypeOf(zr)

	if !firstResult.AssignableTo(rt) {
		return fmt.Errorf("expected return value %T, but got %v", zr, firstResult.String())
	}

	return nil
}
