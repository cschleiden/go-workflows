package workflow

import (
	"time"

	"github.com/cschleiden/go-dt/internal/sync"
)

func Sleep(ctx sync.Context, d time.Duration) error {
	return ScheduleTimer(ctx, d).Get(ctx, nil)
}
