package workflow

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func Sleep(ctx sync.Context, d time.Duration) error {
	span := tracing.Tracer(ctx).Start("Sleep",
		trace.WithAttributes(attribute.Int64("duration_s", int64(d/time.Second))))
	defer span.End()

	_, err := ScheduleTimer(ctx, d).Get(ctx)

	return err
}
