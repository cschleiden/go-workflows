package workflow

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Sleep sleeps for the given duration.
func Sleep(ctx Context, d time.Duration) error {
	ctx, span := Tracer(ctx).Start(ctx, "Sleep",
		trace.WithAttributes(attribute.Int64(log.DurationKey, int64(d/time.Millisecond))))
	defer span.End()

	_, err := ScheduleTimer(ctx, d).Get(ctx)

	return err
}
