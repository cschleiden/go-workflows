package redis

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/cschleiden/go-workflows/log"
	"github.com/go-redis/redis/v8"
)

func Benchmark_RedisBackend(b *testing.B) {
	if testing.Short() {
		b.Skip()
	}

	test.SimpleWorkflowBenchmark(b, getCreateBackend(true), nil)
}

func Test_RedisBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.BackendTest(t, getCreateBackend(false), nil)
}

func Test_EndToEndRedisBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.EndToEndBackendTest(t, getCreateBackend(false), nil)
}

func getCreateBackend(ignoreLog bool) func() backend.Backend {
	return func() backend.Backend {
		address := "localhost:6379"
		user := ""
		password := "RedisPassw0rd"

		// Flush database
		client := redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:    []string{address},
			Username: user,
			Password: password,
			DB:       0,
		})

		if err := client.FlushDB(context.Background()).Err(); err != nil {
			panic(err)
		}

		options := []RedisBackendOption{
			WithBlockTimeout(time.Millisecond * 10),
		}

		if ignoreLog {
			options = append(options, WithBackendOptions(backend.WithLogger(&nullLogger{})))
		}

		b, err := NewRedisBackend(address, user, password, 0, options...)
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
