package tracing

import (
	"context"
	"reflect"
	"time"
	"unsafe"

	"go.opentelemetry.io/otel/trace"
)

func SpanWithStartTime(
	ctx context.Context, tracer trace.Tracer, name string, spanID trace.SpanID, startTime time.Time, opts ...trace.SpanStartOption) trace.Span {

	opts = append(opts, trace.WithTimestamp(startTime), trace.WithSpanKind(trace.SpanKindConsumer))
	_, span := tracer.Start(ctx,
		name,
		opts...,
	)

	SetSpanID(span, spanID)

	return span
}

func GetNewSpanID(tracer trace.Tracer) trace.SpanID {
	// Create workflow span. We pass the name here, but in the end we only use the span ID.
	// The tracer doesn't expose the span ID generator that's being used, so we use this
	// ugly workaround here.
	_, span := tracer.Start(context.Background(), "canceled-span", trace.WithSpanKind(trace.SpanKindInternal))
	cancelSpan(span) // We don't actually want to send this span
	span.End()

	return span.SpanContext().SpanID()
}

func SetSpanID(span trace.Span, sid trace.SpanID) {
	sc := span.SpanContext()
	sc = sc.WithSpanID(sid)
	setSpanContext(span, sc)
}

func cancelSpan(span trace.Span) {
	sc := span.SpanContext()
	if sc.IsSampled() {
		sc = sc.WithTraceFlags(trace.TraceFlags(0)) // Remove the sampled flag
		setSpanContext(span, sc)
	}
}

func setSpanContext(span trace.Span, sc trace.SpanContext) {
	spanP := reflect.ValueOf(span)
	spanV := reflect.Indirect(spanP)
	field := spanV.FieldByName("spanContext")

	// noop or nonrecording spans store their spanContext in `sc`, but we ignore
	// those for our purposes here.
	if !field.IsValid() || field.IsZero() {
		return
	}

	setUnexportedField(field, sc)
}

func setUnexportedField(field reflect.Value, value any) {
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).
		Elem().
		Set(reflect.ValueOf(value))
}
