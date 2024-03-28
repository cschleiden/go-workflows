package tracing

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"go.opentelemetry.io/otel/sdk/trace"
	tc "go.opentelemetry.io/otel/trace"
)

func Test_Foo(t *testing.T) {
	p := trace.NewTracerProvider()
	_, s := p.Tracer("foo").Start(context.Background(), "foo")

	sc := s.SpanContext()
	fmt.Println(sc.SpanID())

	setSpanID(s, tc.SpanID([]byte("foobar12")))

	sc = s.SpanContext()
	fmt.Println(sc.SpanID())
}

func setSpanID(span tc.Span, sid tc.SpanID) {
	sc := span.SpanContext()

	setUnexportedField(reflect.ValueOf(&sc).Elem().FieldByName("spanID"), tc.SpanID([]byte("foobar12")))
	setUnexportedField(reflect.ValueOf(span).Elem().FieldByName("spanContext"), sc)
}

func setUnexportedField(field reflect.Value, value interface{}) {
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).
		Elem().
		Set(reflect.ValueOf(value))
}
