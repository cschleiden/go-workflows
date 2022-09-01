package metrics

import (
	"time"

	m "github.com/cschleiden/go-workflows/metrics"
)

type noopMetricsClient struct {
}

func NewNoopMetricsClient() *noopMetricsClient {
	return &noopMetricsClient{}
}

var _ m.Client = (*noopMetricsClient)(nil)

func (*noopMetricsClient) Counter(name string, tags m.Tags, value float64) {
}

// Distribution implements metrics.Client
func (*noopMetricsClient) Distribution(name string, tags m.Tags, value float64) {
}

// Timing implements metrics.Client
func (*noopMetricsClient) Timing(name string, tags m.Tags, duration time.Duration) {
}

// WithTags implements metrics.Client
func (nmc *noopMetricsClient) WithTags(tags m.Tags) m.Client {
	return nmc
}
