package metrics

import (
	"time"

	"github.com/cschleiden/go-workflows/backend/metrics"
)

type Timer struct {
	client metrics.Client
	start  time.Time
	name   string
	tags   metrics.Tags
}

func NewTimer(client metrics.Client, name string, tags metrics.Tags) *Timer {
	return &Timer{
		client: client,
		start:  time.Now(),
		name:   name,
		tags:   tags,
	}
}

// Stop the timer and send the elapsed time as milliseconds as a distribution metric
func (t *Timer) Stop() {
	elapsed := time.Since(t.start)
	t.client.Distribution(t.name, t.tags, float64(elapsed/time.Millisecond))
}
