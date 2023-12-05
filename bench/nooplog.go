package main

import (
	"context"
	"log/slog"
)

type nullHandler struct {
}

// Enabled implements slog.Handler.
func (*nullHandler) Enabled(context.Context, slog.Level) bool {
	return false
}

// Handle implements slog.Handler.
func (*nullHandler) Handle(context.Context, slog.Record) error {
	return nil
}

// WithAttrs implements slog.Handler.
func (nl *nullHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return nl
}

// WithGroup implements slog.Handler.
func (nl *nullHandler) WithGroup(name string) slog.Handler {
	return nl
}

var _ slog.Handler = (*nullHandler)(nil)
