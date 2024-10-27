package tracing

import (
	"github.com/cschleiden/go-workflows/internal/sync"
	"go.opentelemetry.io/otel/trace"
)

type spanContextKeyType int

const spanKey spanContextKeyType = iota

func ContextWithSpan(ctx sync.Context, span trace.Span) sync.Context {
	return sync.WithValue(ctx, spanKey, span)
}

func SpanFromContext(ctx sync.Context) trace.Span {
	if span, ok := ctx.Value(spanKey).(trace.Span); ok {
		return span
	}

	return nil
}
