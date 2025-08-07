package workflowerrors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_getErrorType_stringError(t *testing.T) {
	require.Empty(t, getErrorType(errors.New("test")))
}

func Test_getErrorType_error(t *testing.T) {
	err := FromError(errors.New("test"))

	etype := getErrorType(err)
	require.Equal(t, "Error", etype) // TODO: Do we want a different type here?
}

type CustomError struct {
	msg string
}

func (ce *CustomError) Error() string {
	return ce.msg
}

func Test_getErrorType_custom(t *testing.T) {
	ce := &CustomError{msg: "test"}

	etype := getErrorType(ce)
	require.Equal(t, "CustomError", etype)
}
