package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cschleiden/go-workflows/pkg/core"
)

type WorkflowExecutorCache interface {
	Store(ctx context.Context, instance core.WorkflowInstance, workflow WorkflowExecutor) error
	Get(ctx context.Context, instance core.WorkflowInstance) (WorkflowExecutor, bool, error)
	StartEviction(ctx context.Context)
}

type workflowExecutorCache struct {
	options WorkflowExecutorCacheOptions
	t       *time.Ticker
	mu      *sync.Mutex
	cache   map[string]*workflowExecutorCacheEntry
}

type workflowExecutorCacheEntry struct {
	executor   WorkflowExecutor
	lastAccess time.Time
}

type WorkflowExecutorCacheOptions struct {
	// CacheDuration is the duration after which a workflow executor is removed from the cache.
	CacheDuration time.Duration
}

var DefaultWorkflowExecutorCacheOptions = WorkflowExecutorCacheOptions{
	CacheDuration: 30 * time.Second,
}

func NewWorkflowExecutorCache(options WorkflowExecutorCacheOptions) WorkflowExecutorCache {
	c := workflowExecutorCache{
		options: options,
		t:       time.NewTicker(options.CacheDuration),
		mu:      &sync.Mutex{},
		cache:   make(map[string]*workflowExecutorCacheEntry),
	}

	return &c
}

func (c *workflowExecutorCache) Store(ctx context.Context, instance core.WorkflowInstance, executor WorkflowExecutor) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[getKey(instance)] = &workflowExecutorCacheEntry{
		executor:   executor,
		lastAccess: time.Now(),
	}

	return nil
}

func (c *workflowExecutorCache) Get(ctx context.Context, instance core.WorkflowInstance) (WorkflowExecutor, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.cache[getKey(instance)]; ok {
		entry.lastAccess = time.Now()
		return entry.executor, true, nil
	}

	return nil, false, nil
}

func (c *workflowExecutorCache) StartEviction(ctx context.Context) {
	for {
		select {
		case <-c.t.C:
			c.mu.Lock()

			cutoff := time.Now().Add(-c.options.CacheDuration)

			// Check cache entries for eviction
			for instance, entry := range c.cache {
				if entry.lastAccess.Before(cutoff) {
					entry.executor.Close()

					delete(c.cache, instance)
				}
			}

			c.mu.Unlock()

		case <-ctx.Done():
			return
		}
	}
}

func getKey(instance core.WorkflowInstance) string {
	return fmt.Sprintf("%s-%s", instance.GetInstanceID(), instance.GetExecutionID())
}
