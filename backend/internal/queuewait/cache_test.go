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
	byQueue map[uuid.UUID]time.Duration
	err     error
	calls   int
}

func (c *countingEstimator) EstimateByModeQueue(_ context.Context, _ []uuid.UUID, _ time.Time) (map[uuid.UUID]time.Duration, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return c.byQueue, nil
}

func TestCacheAnswersEveryQueueOnThePageFromOneEstimate(t *testing.T) {
	first, second, third := uuid.New(), uuid.New(), uuid.New()
	est := &countingEstimator{byQueue: map[uuid.UUID]time.Duration{
		first:  10 * time.Second,
		second: 20 * time.Second,
		third:  30 * time.Second,
	}}
	cache := NewCache(est, time.Minute)
	ctx := context.Background()

	// Three cards on a page, each resolving its own field.
	for _, queueID := range []uuid.UUID{first, second, third} {
		if _, _, err := cache.For(ctx, queueID); err != nil {
			t.Fatalf("For(%v): %v", queueID, err)
		}
	}

	if est.calls != 1 {
		t.Errorf("estimated %d times for 3 queues, want 1 — that is the N+1 this cache exists to prevent", est.calls)
	}
}

func TestCacheReportsAQueueWithNoEstimate(t *testing.T) {
	known, unknown := uuid.New(), uuid.New()
	est := &countingEstimator{byQueue: map[uuid.UUID]time.Duration{known: 12 * time.Second}}
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
	queueID := uuid.New()
	est := &countingEstimator{byQueue: map[uuid.UUID]time.Duration{queueID: 12 * time.Second}}
	cache := NewCache(est, time.Minute)

	wait, ok, err := cache.For(context.Background(), queueID)
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
	queueID := uuid.New()
	est := &countingEstimator{byQueue: map[uuid.UUID]time.Duration{queueID: time.Second}}
	cache := NewCache(est, time.Nanosecond)
	ctx := context.Background()

	if _, _, err := cache.For(ctx, queueID); err != nil {
		t.Fatalf("first For: %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, _, err := cache.For(ctx, queueID); err != nil {
		t.Fatalf("second For: %v", err)
	}

	if est.calls != 2 {
		t.Errorf("estimated %d times, want 2 — an expired snapshot must be refreshed", est.calls)
	}
}

func TestCachePropagatesEstimatorFailure(t *testing.T) {
	wantErr := errors.New("database is down")
	cache := NewCache(&countingEstimator{err: wantErr}, time.Minute)

	if _, _, err := cache.For(context.Background(), uuid.New()); !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want it to wrap %v", err, wantErr)
	}
}
