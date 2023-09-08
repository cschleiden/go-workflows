package tester

import (
	"log/slog"
	"strings"
)

type debugLogger struct {
	b      *strings.Builder
	logger *slog.Logger
}

func newDebugLogger() *debugLogger {
	b := &strings.Builder{}

	logger := slog.New(slog.NewTextHandler(b, nil))

	return &debugLogger{
		b:      b,
		logger: logger,
	}
}

func (dl *debugLogger) hasLine(msg string) bool {
	s := dl.b.String()

	return strings.Contains(s, msg)
}
