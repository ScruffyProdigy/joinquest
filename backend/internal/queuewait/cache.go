package queuewait

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Cache holds one whole-catalog snapshot of wait estimates for a short TTL.
//
// Every mode card on a page asks for its own badge, and catalog pages are
// polled, so without this the same history aggregate would run once per card
// per poll. It is never a source of truth — an estimate that is a few seconds
// stale reads the same to a player, and the underlying number is a median over
// days.
//
// Deliberately the same shape as catalogstats.Cache, which solves this exact
// problem for live player counts.
type Cache struct {
	estimator Estimator
	ttl       time.Duration

	mu        sync.Mutex
	snapshot  map[uuid.UUID]time.Duration
	expiresAt time.Time
}

func NewCache(estimator Estimator, ttl time.Duration) *Cache {
	return &Cache{estimator: estimator, ttl: ttl}
}

// Get returns the current snapshot, refreshing it when the TTL has run out.
func (c *Cache) Get(ctx context.Context) (map[uuid.UUID]time.Duration, error) {
	now := time.Now()

	c.mu.Lock()
	if c.snapshot != nil && now.Before(c.expiresAt) {
		snapshot := c.snapshot
		c.mu.Unlock()
		return snapshot, nil
	}
	c.mu.Unlock()

	// Refreshing outside the lock lets a burst of concurrent requests overlap on
	// one refresh rather than queue behind it; the loser just overwrites with
	// equally fresh numbers.
	fetched, err := c.estimator.EstimateByModeQueue(ctx, nil, time.Now())
	if err != nil {
		return nil, err
	}
	if fetched == nil {
		fetched = map[uuid.UUID]time.Duration{}
	}

	c.mu.Lock()
	c.snapshot = fetched
	c.expiresAt = time.Now().Add(c.ttl)
	c.mu.Unlock()

	return fetched, nil
}

// For returns one queue's estimate. The bool is false when that queue has no
// estimate worth showing, which is not an error.
func (c *Cache) For(ctx context.Context, modeQueueID uuid.UUID) (time.Duration, bool, error) {
	snapshot, err := c.Get(ctx)
	if err != nil {
		return 0, false, err
	}
	wait, ok := snapshot[modeQueueID]
	return wait, ok, nil
}
