package gameclient

import (
	"context"
	"sync"
	"time"
)

// cacheEntry holds a cached eligibility result and a generation token to prevent stale timers
// from evicting freshly-fetched entries that raced with an older fetch.
type cacheEntry struct {
	data  map[string]ModeEligibility
	token uint64
}

// EligibilityCache caches FetchModeEligibility results briefly per (gameID, lobbyUserID).
// It is never a source of truth — just enough to absorb rapid re-renders/polling without
// hammering the game server on every panel load.
type EligibilityCache struct {
	client *Client
	ttl    time.Duration

	mu      sync.Mutex
	items   map[string]cacheEntry
	nextTok uint64
}

func NewEligibilityCache(client *Client, ttl time.Duration) *EligibilityCache {
	return &EligibilityCache{
		client: client,
		ttl:    ttl,
		items:  make(map[string]cacheEntry),
	}
}

func eligibilityCacheKey(gameID, lobbyUserID string) string {
	return gameID + "|" + lobbyUserID
}

// Get returns cached eligibility for (gameID, lobbyUserID), fetching from apiBaseURL on a miss.
func (c *EligibilityCache) Get(ctx context.Context, gameID, apiBaseURL, lobbyUserID string) (map[string]ModeEligibility, error) {
	key := eligibilityCacheKey(gameID, lobbyUserID)

	c.mu.Lock()
	if entry, ok := c.items[key]; ok {
		c.mu.Unlock()
		return entry.data, nil
	}
	c.mu.Unlock()

	fetched, err := c.client.FetchModeEligibility(ctx, apiBaseURL, lobbyUserID)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.nextTok++
	token := c.nextTok
	c.items[key] = cacheEntry{data: fetched, token: token}
	c.mu.Unlock()
	time.AfterFunc(c.ttl, func() {
		c.mu.Lock()
		if entry, ok := c.items[key]; ok && entry.token == token {
			delete(c.items, key)
		}
		c.mu.Unlock()
	})

	return fetched, nil
}
