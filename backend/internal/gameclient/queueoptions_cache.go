package gameclient

import (
	"context"
	"sync"
	"time"
)

type queueOptionsEntry struct {
	roster *QueueOptionRoster
	token  uint64
}

// QueueOptionsCache caches FetchQueueOptions briefly per
// (gameID, lobbyUserID, modeKey). It exists so that rendering the picker and
// then validating the join a second later do not both hit the game — the roster
// the player is validated against is the one they were shown.
//
// Failures are never cached: a game that recovers should become joinable on the
// next attempt, not after a TTL.
type QueueOptionsCache struct {
	client *Client
	ttl    time.Duration

	mu      sync.Mutex
	items   map[string]queueOptionsEntry
	nextTok uint64
}

func NewQueueOptionsCache(client *Client, ttl time.Duration) *QueueOptionsCache {
	return &QueueOptionsCache{
		client: client,
		ttl:    ttl,
		items:  make(map[string]queueOptionsEntry),
	}
}

func queueOptionsCacheKey(gameID, lobbyUserID, modeKey string) string {
	return gameID + "|" + lobbyUserID + "|" + modeKey
}

// Get returns this player's roster for a mode, fetching from apiBaseURL on a miss.
func (c *QueueOptionsCache) Get(ctx context.Context, gameID, apiBaseURL, lobbyUserID, modeKey, fallbackGroupKey string) (*QueueOptionRoster, error) {
	key := queueOptionsCacheKey(gameID, lobbyUserID, modeKey)

	c.mu.Lock()
	if entry, ok := c.items[key]; ok {
		c.mu.Unlock()
		return entry.roster, nil
	}
	c.mu.Unlock()

	fetched, err := c.client.FetchQueueOptions(ctx, apiBaseURL, lobbyUserID, modeKey, fallbackGroupKey)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.nextTok++
	token := c.nextTok
	c.items[key] = queueOptionsEntry{roster: fetched, token: token}
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
