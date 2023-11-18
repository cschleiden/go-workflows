package workflowerrors

import (
	"bytes"
	"runtime"

	goerrors "github.com/go-errors/errors"
)

const MaxStackDepth = 50

// stack returns a stacktrace formatted via go-errors/errors
func stack(skip int) string {
	// get stack
	stack := make([]uintptr, MaxStackDepth)
	length := runtime.Callers(2+skip, stack[:])

	// trim
	stack = stack[:length]

	frames := make([]goerrors.StackFrame, len(stack))
	for i, pc := range stack {
		frames[i] = goerrors.NewStackFrame(pc)
	}

	// Convert frames to trace
	buf := bytes.Buffer{}

	for _, frame := range frames {
		buf.WriteString(frame.String())
	}

	return buf.String()
}
