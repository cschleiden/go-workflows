package fn

import (
	"reflect"
	"runtime"
	"strings"
)

// Name returns the name of the function.
func Name(f any) string {
	// Adapted from https://stackoverflow.com/a/7053871
	fnName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()

	s := strings.Split(fnName, ".")
	fnName = s[len(s)-1]

	return strings.TrimSuffix(fnName, "-fm")
}
