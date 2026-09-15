package queuewait

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// countingCountSource reports fixed waiting counts and counts how often it was
// asked, so a test can prove the cache is actually absorbing repeat reads.
type countingCountSource struct {
	byQueue map[QueueKey]int
	err     error
	calls   int
}

func (c *countingCountSource) load(_ context.Context) (map[QueueKey]int, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return c.byQueue, nil
}

func TestCountCacheAnswersEveryQueueOnThePageFromOneCount(t *testing.T) {
	first, second, third := uuid.New(), uuid.New(), uuid.New()
	src := &countingCountSource{byQueue: map[QueueKey]int{
		{ModeQueueID: first}:  3,
		{ModeQueueID: second}: 5,
		{ModeQueueID: third}:  8,
	}}
	cache := NewCountCache(src.load, time.Minute)
	ctx := context.Background()

	// Three cards on a page, each resolving its own field.
	for _, queueID := range []uuid.UUID{first, second, third} {
		if _, err := cache.For(ctx, queueID); err != nil {
			t.Fatalf("For(%v): %v", queueID, err)
		}
	}

	if src.calls != 1 {
		t.Errorf("counted %d times for 3 queues, want 1 — that is the N+1 this cache exists to prevent", src.calls)
	}
}

func TestCountCacheReadsAnEmptyQueueAsZero(t *testing.T) {
	busy, empty := uuid.New(), uuid.New()
	src := &countingCountSource{byQueue: map[QueueKey]int{{ModeQueueID: busy}: 4}}
	cache := NewCountCache(src.load, time.Minute)

	got, err := cache.For(context.Background(), empty)
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	// A queue nobody is waiting in is absent from the aggregate, because the
	// only rows it can return are rows that exist. That absence is a real zero,
	// not missing data, and must not surface as an error or a null.
	if got != 0 {
		t.Errorf("got %d for a queue with nobody waiting, want 0", got)
	}
}

func TestCountCacheTotalsACompositionModeAcrossItsRoles(t *testing.T) {
	queueID := uuid.New()
	src := &countingCountSource{byQueue: map[QueueKey]int{
		{ModeQueueID: queueID, QueuePath: "tank"}:    1,
		{ModeQueueID: queueID, QueuePath: "damage"}:  9,
		{ModeQueueID: queueID, QueuePath: "support"}: 2,
		{ModeQueueID: uuid.New(), QueuePath: "tank"}: 100,
	}}
	cache := NewCountCache(src.load, time.Minute)

	got, err := cache.For(context.Background(), queueID)
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	// Every waiting player sits in exactly one role's line, so the mode's total
	// is the sum of its lines — and only its own.
	if got != 12 {
		t.Errorf("got %d, want 12 — this mode's three roles summed, without the other queue's", got)
	}
}

func TestCountCachePathsForReportsEveryRoleWithAQueue(t *testing.T) {
	queueID := uuid.New()
	src := &countingCountSource{byQueue: map[QueueKey]int{
		{ModeQueueID: queueID, QueuePath: "tank"}:    1,
		{ModeQueueID: queueID, QueuePath: "damage"}:  9,
		{ModeQueueID: uuid.New(), QueuePath: "tank"}: 100,
	}}
	cache := NewCountCache(src.load, time.Minute)

	got, err := cache.PathsFor(context.Background(), queueID)
	if err != nil {
		t.Fatalf("PathsFor: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d paths, want this queue's 2", len(got))
	}
	if got["tank"] != 1 || got["damage"] != 9 {
		t.Errorf("got tank=%d damage=%d, want 1 and 9", got["tank"], got["damage"])
	}
}

func TestCountCachePathsForIgnoresTheUnsplitLine(t *testing.T) {
	queueID := uuid.New()
	src := &countingCountSource{byQueue: map[QueueKey]int{{ModeQueueID: queueID}: 6}}
	cache := NewCountCache(src.load, time.Minute)

	got, err := cache.PathsFor(context.Background(), queueID)
	if err != nil {
		t.Fatalf("PathsFor: %v", err)
	}
	// A mode that does not split by role has no per-role breakdown to give. Its
	// six players are reported by For, not here.
	if len(got) != 0 {
		t.Errorf("got %d paths, want none — this mode does not split by role", len(got))
	}
}

func TestCountCacheRefreshesOnceTheTTLHasRunOut(t *testing.T) {
	queueID := uuid.New()
	src := &countingCountSource{byQueue: map[QueueKey]int{{ModeQueueID: queueID}: 1}}
	cache := NewCountCache(src.load, time.Nanosecond)
	ctx := context.Background()

	if _, err := cache.For(ctx, queueID); err != nil {
		t.Fatalf("first For: %v", err)
	}
	// A player joining has to show up promptly, so an expired snapshot is
	// refetched rather than served stale.
	src.byQueue = map[QueueKey]int{{ModeQueueID: queueID}: 2}
	time.Sleep(time.Millisecond)

	got, err := cache.For(ctx, queueID)
	if err != nil {
		t.Fatalf("second For: %v", err)
	}
	if got != 2 {
		t.Errorf("got %d after the TTL ran out, want the refreshed 2", got)
	}
	if src.calls != 2 {
		t.Errorf("counted %d times, want 2 — one per expired snapshot", src.calls)
	}
}

func TestCountCachePropagatesSourceFailure(t *testing.T) {
	wantErr := errors.New("database is down")
	cache := NewCountCache((&countingCountSource{err: wantErr}).load, time.Minute)
	ctx := context.Background()

	if _, err := cache.For(ctx, uuid.New()); !errors.Is(err, wantErr) {
		t.Errorf("For got error %v, want it to wrap %v — a broken lookup must not read as an empty queue", err, wantErr)
	}
	if _, err := cache.PathsFor(ctx, uuid.New()); !errors.Is(err, wantErr) {
		t.Errorf("PathsFor got error %v, want it to wrap %v", err, wantErr)
	}
}

func TestCountCacheToleratesASourceThatReturnsNothing(t *testing.T) {
	cache := NewCountCache(func(context.Context) (map[QueueKey]int, error) { return nil, nil }, time.Minute)

	got, err := cache.For(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	// Nobody waiting anywhere in the catalog is a normal quiet moment, not a
	// reason to refetch on every read.
	if got != 0 {
		t.Errorf("got %d, want 0 when nobody in the catalog is waiting", got)
	}
}
