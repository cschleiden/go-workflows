package workflow

import (
	"time"

	"github.com/cschleiden/go-workflows/internal/sync"
)

func Sleep(ctx sync.Context, d time.Duration) error {
	_, err := ScheduleTimer(ctx, d).Get(ctx)
	return err
}
