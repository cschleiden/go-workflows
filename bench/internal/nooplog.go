package internal

import (
	"context"
	"log/slog"
)

type NullHandler struct {
}

// Enabled implements slog.Handler.
func (*NullHandler) Enabled(context.Context, slog.Level) bool {
	return false
}

// Handle implements slog.Handler.
func (*NullHandler) Handle(context.Context, slog.Record) error {
	return nil
}

// WithAttrs implements slog.Handler.
func (nl *NullHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return nl
}

// WithGroup implements slog.Handler.
func (nl *NullHandler) WithGroup(name string) slog.Handler {
	return nl
}

var _ slog.Handler = (*NullHandler)(nil)
