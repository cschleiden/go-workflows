package workflow

import (
	"time"
)

// Sleep sleeps for the given duration.
func Sleep(ctx Context, d time.Duration) error {
	_, err := ScheduleTimer(ctx, d, WithTimerName("Sleep")).Get(ctx)

	return err
}
