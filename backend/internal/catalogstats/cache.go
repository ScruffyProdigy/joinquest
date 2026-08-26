// Package catalogstats caches the live player counts shown on catalog cards.
package catalogstats

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Counts is how many players a game has right now: seated in a live session, and waiting in
// its queues.
type Counts struct {
	Playing int
	Queued  int
}

// Source loads counts for every game that has any, keyed by game id. Games with no activity
// are left out of the map.
type Source func(ctx context.Context) (map[uuid.UUID]Counts, error)

// Cache holds one whole-catalog snapshot of live counts for a short TTL.
//
// Every card on the catalog page asks for its own numbers, and the page is polled, so without
// this the same aggregate would run dozens of times a second. It is never a source of truth —
// a count that is a few seconds stale reads the same to a player.
type Cache struct {
	source Source
	ttl    time.Duration

	mu        sync.Mutex
	snapshot  map[uuid.UUID]Counts
	expiresAt time.Time
}

func NewCache(source Source, ttl time.Duration) *Cache {
	return &Cache{source: source, ttl: ttl}
}

// Get returns the current snapshot, refreshing it when the TTL has run out.
func (c *Cache) Get(ctx context.Context) (map[uuid.UUID]Counts, error) {
	now := time.Now()

	c.mu.Lock()
	if c.snapshot != nil && now.Before(c.expiresAt) {
		snapshot := c.snapshot
		c.mu.Unlock()
		return snapshot, nil
	}
	c.mu.Unlock()

	// Refreshing outside the lock lets a burst of concurrent requests overlap on one refresh
	// rather than queue behind it; the loser just overwrites with equally fresh numbers.
	fetched, err := c.source(ctx)
	if err != nil {
		return nil, err
	}
	if fetched == nil {
		fetched = map[uuid.UUID]Counts{}
	}

	c.mu.Lock()
	c.snapshot = fetched
	c.expiresAt = time.Now().Add(c.ttl)
	c.mu.Unlock()

	return fetched, nil
}

// For returns one game's counts, defaulting to zeroes when the game has no activity.
func (c *Cache) For(ctx context.Context, gameID uuid.UUID) (Counts, error) {
	snapshot, err := c.Get(ctx)
	if err != nil {
		return Counts{}, err
	}
	return snapshot[gameID], nil
}
