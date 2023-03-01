package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/log"
	"github.com/redis/go-redis/v9"
)

const (
	address  = "localhost:6379"
	user     = ""
	password = "RedisPassw0rd"
)

func Benchmark_RedisBackend(b *testing.B) {
	if testing.Short() {
		b.Skip()
	}

	client := getClient()
	setup := getCreateBackend(client, true)

	test.SimpleWorkflowBenchmark(b, setup, nil)
}

func Test_RedisBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	client := getClient()
	setup := getCreateBackend(client, false)

	test.BackendTest(t, setup, nil)
}

func Test_EndToEndRedisBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	client := getClient()
	setup := getCreateBackend(client, false)

	test.EndToEndBackendTest(t, setup, nil)
}

func getClient() redis.UniversalClient {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{address},
		Username: user,
		Password: password,
		DB:       0,
	})

	return client
}

func getCreateBackend(client redis.UniversalClient, ignoreLog bool) func() test.TestBackend {
	return func() test.TestBackend {
		// Flush database
		if err := client.FlushDB(context.Background()).Err(); err != nil {
			panic(err)
		}

		r, err := client.Keys(context.Background(), "*").Result()
		if err != nil {
			panic(err)
		}

		if len(r) > 0 {
			panic("Keys should've been empty" + strings.Join(r, ", "))
		}

		options := []RedisBackendOption{
			WithBlockTimeout(time.Millisecond * 10),
		}

		if ignoreLog {
			options = append(options, WithBackendOptions(backend.WithLogger(&nullLogger{})))
		}

		b, err := NewRedisBackend(client, options...)
		if err != nil {
			panic(err)
		}

		return b
	}
}

type nullLogger struct {
	defaultFields []interface{}
}

// Debug implements log.Logger
func (*nullLogger) Debug(msg string, fields ...interface{}) {
}

// Error implements log.Logger
func (*nullLogger) Error(msg string, fields ...interface{}) {
}

// Panic implements log.Logger
func (*nullLogger) Panic(msg string, fields ...interface{}) {
}

// Warn implements log.Logger
func (*nullLogger) Warn(msg string, fields ...interface{}) {
}

// With implements log.Logger
func (nl *nullLogger) With(fields ...interface{}) log.Logger {
	return nl
}

var _ log.Logger = (*nullLogger)(nil)

var _ test.TestBackend = (*redisBackend)(nil)

// GetFutureEvents
func (rb *redisBackend) GetFutureEvents(ctx context.Context) ([]*history.Event, error) {
	r, err := rb.rdb.ZRangeByScore(ctx, futureEventsKey(), &redis.ZRangeBy{
		Min: "-inf",
		Max: "+inf",
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("getting future events: %w", err)
	}

	events := make([]*history.Event, 0)

	for _, eventID := range r {
		eventStr, err := rb.rdb.HGet(ctx, eventID, "event").Result()
		if err != nil {
			return nil, fmt.Errorf("getting event %v: %w", eventID, err)
		}

		var event *history.Event
		if err := json.Unmarshal([]byte(eventStr), &event); err != nil {
			return nil, fmt.Errorf("unmarshaling event %v: %w", eventID, err)
		}

		events = append(events, event)
	}

	return events, nil
}
