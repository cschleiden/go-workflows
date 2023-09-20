package fn

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

func FuncName(i any) string {
	// Adapted from https://stackoverflow.com/a/7053871
	fnName := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
	return strings.TrimSuffix(fnName, "-fm")
}

func StructName(v any) string {
	t := reflect.TypeOf(v)

	tt := t
	pkgPath := tt.PkgPath()
	for pkgPath == "" && tt.Kind() == reflect.Pointer {
		tt = tt.Elem()
		pkgPath = tt.PkgPath()
	}
	if pkgPath != "" {
		pkgPath += "."
	}

	tokens := strings.Split(t.String(), ".")
	typeName := strings.TrimPrefix(tokens[len(tokens)-1], "*")
	if t.Kind() == reflect.Pointer {
		typeName = fmt.Sprintf("*%v", typeName)
		if pkgPath != "" {
			typeName = fmt.Sprintf("(%v)", typeName)
		}
	}

	return pkgPath + typeName
}
