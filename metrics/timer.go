package metrics

import (
	"time"
)

type timer struct {
	client Client
	start  time.Time
	name   string
	tags   Tags
}

func Timer(client Client, name string, tags Tags) *timer {
	return &timer{
		client: client,
		start:  time.Now(),
		name:   name,
		tags:   tags,
	}
}

// Stop the timer and send the elapsed time as milliseconds as a distribution metric
func (t *timer) Stop() {
	elapsed := time.Since(t.start)
	t.client.Distribution(t.name, t.tags, float64(elapsed/time.Millisecond))
}
