package metrics

import "time"

type Tags map[string]string

type Client interface {
	// Counter records a value at a point in time.
	Counter(name string, tags Tags, value int64)

	// Distribution records a value at a point in time.
	Distribution(name string, tags Tags, value float64)

	// Gauge records a value at a point in time.
	Gauge(name string, tags Tags, value int64)

	// Timing records the duration of an event.
	Timing(name string, tags Tags, duration time.Duration)

	// WithTags returns a new client with the given tags applied to all metrics.
	WithTags(tags Tags) Client
}
