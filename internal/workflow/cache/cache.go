package cache

import (
	"context"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/ticctech/go-workflows/internal/core"
	"github.com/ticctech/go-workflows/internal/workflow"
)

type LruCache struct {
	c *ttlcache.Cache[string, workflow.WorkflowExecutor]
}

func NewWorkflowExecutorLRUCache(size int, expiration time.Duration) workflow.ExecutorCache {
	c := ttlcache.New(
		ttlcache.WithCapacity[string, workflow.WorkflowExecutor](uint64(size)),
		ttlcache.WithTTL[string, workflow.WorkflowExecutor](expiration),
	)

	c.OnEviction(func(ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, workflow.WorkflowExecutor]) {
		i.Value().Close()
	})

	return &LruCache{
		c: c,
	}
}

func (lc *LruCache) Get(ctx context.Context, instance *core.WorkflowInstance) (workflow.WorkflowExecutor, bool, error) {
	e := lc.c.Get(getKey(instance))
	if e != nil {
		return e.Value(), true, nil
	}

	return nil, false, nil
}

func (lc *LruCache) Store(ctx context.Context, instance *core.WorkflowInstance, executor workflow.WorkflowExecutor) error {
	lc.c.Set(getKey(instance), executor, ttlcache.DefaultTTL)

	return nil
}

func (lc *LruCache) StartEviction(ctx context.Context) {
	go lc.c.Start()

	<-ctx.Done()

	lc.c.Stop()
}
