package main

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cschleiden/go-workflows/backend/metrics"
)

type store struct {
	counters *sync.Map
	timers   map[string][]time.Duration
}

type memMetrics struct {
	tags metrics.Tags
	s    *store
}

func newMemMetrics() *memMetrics {
	return &memMetrics{
		tags: make(metrics.Tags),
		s: &store{
			counters: &sync.Map{},
			timers:   make(map[string][]time.Duration),
		},
	}
}

func (m *memMetrics) Print() {
	m.s.counters.Range(func(k, v any) bool {
		fmt.Printf("%s: %d\n", k, v)

		return true
	})
}

// Counter implements metrics.Client
func (m *memMetrics) Counter(name string, tags metrics.Tags, value int64) {
	k := key(name, mergeTags(m.tags, tags))

	if v, ok := m.s.counters.Load(k); !ok {
		m.s.counters.Store(k, value)
	} else {
		m.s.counters.Store(k, v.(int64)+value)
	}
}

// Distribution implements metrics.Client
func (m *memMetrics) Distribution(name string, tags metrics.Tags, value float64) {
}

// Gauge implements metrics.Client
func (m *memMetrics) Gauge(name string, tags metrics.Tags, value int64) {
}

// Timing implements metrics.Client
func (m *memMetrics) Timing(name string, tags metrics.Tags, duration time.Duration) {
}

// WithTags implements metrics.Client
func (m *memMetrics) WithTags(tags metrics.Tags) metrics.Client {
	return &memMetrics{
		s:    m.s,
		tags: mergeTags(m.tags, tags),
	}
}

func mergeTags(a, b metrics.Tags) metrics.Tags {
	tags := make(metrics.Tags)
	for k, v := range a {
		tags[k] = v
	}

	for k, v := range b {
		tags[k] = v
	}

	return tags
}

func key(name string, tags metrics.Tags) string {
	t := make([]struct{ Key, Value string }, 0, len(tags))
	for k, v := range tags {
		t = append(t, struct{ Key, Value string }{k, v})
	}

	sort.Slice(t, func(i, j int) bool {
		return t[i].Key < t[j].Key
	})

	var buf bytes.Buffer

	buf.WriteString(name)
	buf.WriteString("[")

	for i, tag := range t {
		if i > 0 {
			buf.WriteString(",")
		}

		buf.WriteString(tag.Key)
		buf.WriteString(":")
		buf.WriteString(tag.Value)
	}

	buf.WriteString("]")

	return buf.String()
}

var _ metrics.Client = (*memMetrics)(nil)
