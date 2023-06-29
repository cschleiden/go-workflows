package workflowerrors

import "reflect"

// getErrorType returns the name of the given error type, returns "" for the built-in error type
func getErrorType(err error) string {
	t := reflect.TypeOf(err)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.PkgPath() == "errors" && t.Name() == "errorString" {
		return ""
	}

	return t.Name()
}
