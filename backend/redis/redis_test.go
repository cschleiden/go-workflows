package redis

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/go-redis/redis/v8"
)

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

	b, err := NewRedisBackend(address, user, password, 0, WithBlockTimeout(time.Millisecond*2))
	if err != nil {
		panic(err)
	}

	return b
}
