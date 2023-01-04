package fn

import (
	"reflect"
	"runtime"
	"strings"
)

func Name(i interface{}) string {
	// Adapted from https://stackoverflow.com/a/7053871
	return strings.TrimSuffix(runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name(), "-fm")
}
