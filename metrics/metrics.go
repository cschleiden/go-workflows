package metrics

import "time"

type Tags map[string]string

type Client interface {
	Counter(name string, tags Tags, value int64)

	Distribution(name string, tags Tags, value float64)

	Gauge(name string, tags Tags, value int64)

	Timing(name string, tags Tags, duration time.Duration)

	WithTags(tags Tags) Client
}
