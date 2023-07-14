package workflowerrors

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_stack(t *testing.T) {
	fn := func() {
		s := stack(1)
		require.NotContains(t, s, "Test_stack.func1")
	}

	foo(fn)
}

func foo(fn func()) {
	bar(fn)
}

func bar(fn func()) {
	fn()
}
