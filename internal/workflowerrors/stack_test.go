package workflowerrors

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_stack(t *testing.T) {
	fn := func() {
		s := stack(1)
		// Check that the stack trace contains the current test function
		require.Contains(t, s, "Test_stack")
		// Check that the stack trace contains the bar function where fn() is called
		require.Contains(t, s, "bar")
	}

	foo(fn)
}

func foo(fn func()) {
	bar(fn)
}

func bar(fn func()) {
	fn()
}
