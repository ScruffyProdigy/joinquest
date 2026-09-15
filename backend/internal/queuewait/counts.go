package queuewait

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CountSource loads how many players are waiting in every line right now, keyed
// by line. Lines with nobody waiting are left out of the map.
type CountSource func(ctx context.Context) (map[QueueKey]int, error)

// CountCache holds one whole-catalog snapshot of waiting counts for a short TTL.
//
// Every mode card on a game-detail page asks for its own waiting count, and the
// page is polled, so without this the same COUNT(*) would run once per card per
// poll. Deliberately the same shape as Cache next door and catalogstats.Cache,
// which solve this for wait estimates and live player counts.
//
// The TTL here is much shorter than the one wait estimates are served behind,
// and for a reason worth keeping: an estimate is a median over days and barely
// moves minute to minute, while this is a live number that a player expects to
// see themselves in. Joining a queue and not appearing in its count reads as
// broken, so the snapshot has to turn over in seconds.
type CountCache struct {
	source CountSource
	ttl    time.Duration

	mu        sync.Mutex
	snapshot  map[QueueKey]int
	expiresAt time.Time
}

func NewCountCache(source CountSource, ttl time.Duration) *CountCache {
	return &CountCache{source: source, ttl: ttl}
}

// Get returns the current snapshot, refreshing it when the TTL has run out.
func (c *CountCache) Get(ctx context.Context) (map[QueueKey]int, error) {
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
	fetched, err := c.source(ctx)
	if err != nil {
		return nil, err
	}
	if fetched == nil {
		fetched = map[QueueKey]int{}
	}

	c.mu.Lock()
	c.snapshot = fetched
	c.expiresAt = time.Now().Add(c.ttl)
	c.mu.Unlock()

	return fetched, nil
}

// For returns how many players are waiting for one mode queue in total, summed
// across its paths.
//
// Summed rather than read from a mode-wide bucket because a composition mode's
// players are only ever counted inside a role's line. Every waiting row lands in
// exactly one path bucket, so the sum is the mode's real total.
//
// A queue nobody is waiting in reads as zero. That is the honest answer, not
// missing data — an empty line is a fact about the queue, and the only way to
// learn it is to find no rows.
func (c *CountCache) For(ctx context.Context, modeQueueID uuid.UUID) (int, error) {
	snapshot, err := c.Get(ctx)
	if err != nil {
		return 0, err
	}
	total := 0
	for key, waiting := range snapshot {
		if key.ModeQueueID == modeQueueID {
			total += waiting
		}
	}
	return total, nil
}

// PathsFor returns each path of one mode queue that has someone waiting, so a
// card can paint all of a composition mode's roles from one snapshot.
//
// Empty for a mode that does not split by path — its players wait in the single
// unnamed line, which For already reports.
func (c *CountCache) PathsFor(ctx context.Context, modeQueueID uuid.UUID) (map[string]int, error) {
	snapshot, err := c.Get(ctx)
	if err != nil {
		return nil, err
	}
	paths := make(map[string]int)
	for key, waiting := range snapshot {
		if key.ModeQueueID == modeQueueID && key.QueuePath != "" {
			paths[key.QueuePath] = waiting
		}
	}
	return paths, nil
}
