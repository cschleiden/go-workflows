package workflowerrors

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_stack(t *testing.T) {
	fn := func() {
		s := stack(1)
		// In Go 1.24+, anonymous functions are named with .func1, .func2, etc.
		require.Contains(t, s, "Test_stack.func1")
	}

	foo(fn)
}

func foo(fn func()) {
	bar(fn)
}

func bar(fn func()) {
	fn()
}
