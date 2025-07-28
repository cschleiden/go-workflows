package tracing

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/cschleiden/go-workflows/internal/sync"
)

type Context map[string]string

func (wim Context) Get(key string) string {
	return wim[key]
}

func (wim Context) Set(key string, value string) {
	wim[key] = value
}

func (wim Context) Keys() []string {
	r := make([]string, 0, len(wim))

	for k := range wim {
		r = append(r, k)
	}

	return r
}

var propagator propagation.TraceContext

func ContextFromWfCtx(ctx sync.Context) Context {
	span := SpanFromContext(ctx)
	spanCtx := trace.ContextWithSpan(context.Background(), span)
	carrier := make(Context)
	propagator.Inject(spanCtx, carrier)
	return carrier
}

func SpanContextFromContext(ctx context.Context, tctx Context) context.Context {
	return propagator.Extract(ctx, tctx)
}
