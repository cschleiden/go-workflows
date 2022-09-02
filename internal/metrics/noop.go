package metrics

import (
	"time"

	"github.com/cschleiden/go-workflows/metrics"
)

type noopMetricsClient struct {
}

func NewNoopMetricsClient() *noopMetricsClient {
	return &noopMetricsClient{}
}

var _ metrics.Client = (*noopMetricsClient)(nil)

func (*noopMetricsClient) Counter(name string, tags metrics.Tags, value int64) {
}

func (*noopMetricsClient) Distribution(name string, tags metrics.Tags, value float64) {
}

func (*noopMetricsClient) Gauge(name string, tags metrics.Tags, value int64) {

}

func (*noopMetricsClient) Timing(name string, tags metrics.Tags, duration time.Duration) {
}

func (nmc *noopMetricsClient) WithTags(tags metrics.Tags) metrics.Client {
	return nmc
}
