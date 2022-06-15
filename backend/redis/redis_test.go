package redis

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/cschleiden/go-workflows/log"
	"github.com/go-redis/redis/v8"
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

	test.SimpleWorkflowBenchmark(b, setup, func(b backend.Backend) {
		if err := b.(*redisBackend).Close(); err != nil {
			panic(err)
		}
	})
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

func getCreateBackend(client redis.UniversalClient, ignoreLog bool) func() backend.Backend {
	return func() backend.Backend {
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
