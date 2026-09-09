package queuewait

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// countingEstimator reports fixed estimates and counts how often it was asked,
// so a test can prove the cache is actually absorbing repeat reads.
type countingEstimator struct {
	byQueue map[QueueKey]time.Duration
	err     error
	calls   int
}

func (c *countingEstimator) EstimateByQueue(_ context.Context, _ []uuid.UUID, _ time.Time) (map[QueueKey]time.Duration, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return c.byQueue, nil
}

func TestCacheAnswersEveryQueueOnThePageFromOneEstimate(t *testing.T) {
	first := QueueKey{ModeQueueID: uuid.New()}
	second := QueueKey{ModeQueueID: uuid.New(), QueuePath: "tank"}
	third := QueueKey{ModeQueueID: second.ModeQueueID, QueuePath: "damage"}
	est := &countingEstimator{byQueue: map[QueueKey]time.Duration{
		first:  10 * time.Second,
		second: 20 * time.Second,
		third:  30 * time.Second,
	}}
	cache := NewCache(est, time.Minute)
	ctx := context.Background()

	// Three cards on a page, each resolving its own field.
	for _, key := range []QueueKey{first, second, third} {
		if _, _, err := cache.For(ctx, key); err != nil {
			t.Fatalf("For(%v): %v", key, err)
		}
	}

	if est.calls != 1 {
		t.Errorf("estimated %d times for 3 queues, want 1 — that is the N+1 this cache exists to prevent", est.calls)
	}
}

func TestCacheReportsAQueueWithNoEstimate(t *testing.T) {
	known := QueueKey{ModeQueueID: uuid.New()}
	unknown := QueueKey{ModeQueueID: uuid.New()}
	est := &countingEstimator{byQueue: map[QueueKey]time.Duration{known: 12 * time.Second}}
	cache := NewCache(est, time.Minute)

	wait, ok, err := cache.For(context.Background(), unknown)
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if ok {
		t.Errorf("got %v and ok=true for a queue with no history, want ok=false", wait)
	}
}

func TestCacheServesAQueueThatHasAnEstimate(t *testing.T) {
	key := QueueKey{ModeQueueID: uuid.New(), QueuePath: "support"}
	est := &countingEstimator{byQueue: map[QueueKey]time.Duration{key: 12 * time.Second}}
	cache := NewCache(est, time.Minute)

	wait, ok, err := cache.For(context.Background(), key)
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if !ok {
		t.Fatal("got ok=false, want the queue's estimate")
	}
	if wait != 12*time.Second {
		t.Errorf("got %v, want 12s", wait)
	}
}

func TestCacheRefreshesOnceItsSnapshotHasExpired(t *testing.T) {
	key := QueueKey{ModeQueueID: uuid.New()}
	est := &countingEstimator{byQueue: map[QueueKey]time.Duration{key: time.Second}}
	cache := NewCache(est, time.Nanosecond)
	ctx := context.Background()

	if _, _, err := cache.For(ctx, key); err != nil {
		t.Fatalf("first For: %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, _, err := cache.For(ctx, key); err != nil {
		t.Fatalf("second For: %v", err)
	}

	if est.calls != 2 {
		t.Errorf("estimated %d times, want 2 — an expired snapshot must be refreshed", est.calls)
	}
}

func TestCachePropagatesEstimatorFailure(t *testing.T) {
	wantErr := errors.New("database is down")
	cache := NewCache(&countingEstimator{err: wantErr}, time.Minute)

	if _, _, err := cache.For(context.Background(), QueueKey{ModeQueueID: uuid.New()}); !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want it to wrap %v", err, wantErr)
	}
}

func TestCachePathsForReturnsEveryRoleOfOneQueue(t *testing.T) {
	queueID, otherQueueID := uuid.New(), uuid.New()
	est := &countingEstimator{byQueue: map[QueueKey]time.Duration{
		{ModeQueueID: queueID, QueuePath: "tank"}:      8 * time.Second,
		{ModeQueueID: queueID, QueuePath: "damage"}:    240 * time.Second,
		{ModeQueueID: otherQueueID, QueuePath: "tank"}: 99 * time.Second,
		{ModeQueueID: queueID}:                         30 * time.Second,
	}}
	cache := NewCache(est, time.Minute)

	paths, err := cache.PathsFor(context.Background(), queueID)
	if err != nil {
		t.Fatalf("PathsFor: %v", err)
	}

	if paths["tank"] != 8*time.Second {
		t.Errorf("tank got %v, want 8s", paths["tank"])
	}
	if paths["damage"] != 240*time.Second {
		t.Errorf("damage got %v, want 240s", paths["damage"])
	}
	// Another queue's roles, and this queue's own unsplit bucket, are not paths
	// of this queue.
	if len(paths) != 2 {
		t.Errorf("got %d paths (%v), want exactly this queue's two roles", len(paths), paths)
	}
}

func TestCachePathsForCostsNoExtraEstimate(t *testing.T) {
	queueID := uuid.New()
	est := &countingEstimator{byQueue: map[QueueKey]time.Duration{
		{ModeQueueID: queueID, QueuePath: "tank"}: 8 * time.Second,
	}}
	cache := NewCache(est, time.Minute)
	ctx := context.Background()

	if _, err := cache.PathsFor(ctx, queueID); err != nil {
		t.Fatalf("first PathsFor: %v", err)
	}
	if _, err := cache.PathsFor(ctx, queueID); err != nil {
		t.Fatalf("second PathsFor: %v", err)
	}

	if est.calls != 1 {
		t.Errorf("estimated %d times, want 1 — PathsFor reads the same snapshot", est.calls)
	}
}
