package redis

import (
	"context"
	"log"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/go-redis/redis/v8"
)

func NewRedisBackend(address, username, password string, db int, opts ...backend.BackendOption) backend.Backend {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{address},
		Username: username,
		Password: password,
		DB:       db,
	})

	// TODO: Only for dev
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		panic(err)
	}

	rb := &redisBackend{
		rdb:     client,
		options: backend.ApplyOptions(opts...),

		workflowQueue: newQueue(client, "workflows"),
		activityQueue: newQueue(client, "activities"),
	}

	// HACK: Debug recover code
	go func() {
		t := time.NewTicker(time.Second * 1)

		for {
			select {
			case <-t.C:
				res, err := rb.activityQueue.Recover(context.Background())
				if err != nil {
					panic(err)
				}

				log.Println(res)
			}
		}
	}()

	return rb
}

type redisBackend struct {
	rdb     redis.UniversalClient
	options backend.Options

	workflowQueue *queue
	activityQueue *queue
}

func (rb *redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	// TODO: Store signal event

	panic("unimplemented")
}
