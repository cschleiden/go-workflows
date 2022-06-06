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
	test.SimpleWorkflowBenchmark(b, createBackend, nil)
}

func Test_RedisBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.BackendTest(t, createBackend, nil)
}

func Test_EndToEndRedisBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.EndToEndBackendTest(t, createBackend, nil)
}

func createBackend() backend.Backend {
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

	b, err := NewRedisBackend(address, user, password, 0, WithBlockTimeout(time.Millisecond*2), WithBackendOptions(backend.WithLogger(&nullLogger{})))
	if err != nil {
		panic(err)
	}

	return b
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
