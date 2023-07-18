package workflowerrors

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_NewPanicError(t *testing.T) {
	e := func() *PanicError {
		return NewPanicError("test")
	}()

	require.NotContains(t, e.Stack(), "Test_NewPanicError.func1")
	require.NotContains(t, e.Stack(), "NewPanicError")
}
