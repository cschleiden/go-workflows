package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/test"
)

const (
	address  = "localhost:6379"
	user     = ""
	password = "RedisPassw0rd"
)

func Test_RedisBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	client := getClient()
	setup := getCreateBackend(client)

	test.BackendTest(t, setup, nil)
}

func Test_EndToEndRedisBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	client := getClient()
	setup := getCreateBackend(client)

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

func getCreateBackend(client redis.UniversalClient, additionalOptions ...RedisBackendOption) func(options ...backend.BackendOption) test.TestBackend {
	return func(options ...backend.BackendOption) test.TestBackend {
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

		redisOptions := []RedisBackendOption{
			WithBlockTimeout(time.Millisecond * 10),
			WithBackendOptions(options...),
		}

		redisOptions = append(redisOptions, additionalOptions...)

		b, err := NewRedisBackend(client, redisOptions...)
		if err != nil {
			panic(err)
		}

		return b
	}
}

var _ test.TestBackend = (*redisBackend)(nil)

// GetFutureEvents
func (rb *redisBackend) GetFutureEvents(ctx context.Context) ([]*history.Event, error) {
	r, err := rb.rdb.ZRangeByScore(ctx, rb.keys.futureEventsKey(), &redis.ZRangeBy{
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
