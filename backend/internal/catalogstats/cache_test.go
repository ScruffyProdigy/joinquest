package catalogstats

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCacheServesOneSnapshotToManyCards(t *testing.T) {
	gameID := uuid.New()
	calls := 0
	cache := NewCache(func(context.Context) (map[uuid.UUID]Counts, error) {
		calls++
		return map[uuid.UUID]Counts{gameID: {Playing: 4, Queued: 2}}, nil
	}, time.Minute)

	for i := 0; i < 10; i++ {
		got, err := cache.For(context.Background(), gameID)
		if err != nil {
			t.Fatalf("For: %v", err)
		}
		if got.Playing != 4 || got.Queued != 2 {
			t.Fatalf("counts = %+v, want {4 2}", got)
		}
	}
	if calls != 1 {
		t.Fatalf("source called %d times, want 1", calls)
	}
}

func TestCacheRefreshesAfterTTL(t *testing.T) {
	gameID := uuid.New()
	calls := 0
	cache := NewCache(func(context.Context) (map[uuid.UUID]Counts, error) {
		calls++
		return map[uuid.UUID]Counts{gameID: {Playing: calls}}, nil
	}, time.Nanosecond)

	if _, err := cache.For(context.Background(), gameID); err != nil {
		t.Fatalf("For: %v", err)
	}
	time.Sleep(time.Millisecond)
	got, err := cache.For(context.Background(), gameID)
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if got.Playing != 2 {
		t.Fatalf("playing = %d, want the refreshed 2", got.Playing)
	}
}

func TestCacheReportsZeroForAQuietGame(t *testing.T) {
	cache := NewCache(func(context.Context) (map[uuid.UUID]Counts, error) {
		return nil, nil
	}, time.Minute)

	got, err := cache.For(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if got != (Counts{}) {
		t.Fatalf("counts = %+v, want zeroes", got)
	}
}

func TestCacheDoesNotCacheFailures(t *testing.T) {
	calls := 0
	cache := NewCache(func(context.Context) (map[uuid.UUID]Counts, error) {
		calls++
		return nil, errors.New("database is down")
	}, time.Minute)

	for i := 0; i < 2; i++ {
		if _, err := cache.Get(context.Background()); err == nil {
			t.Fatal("expected the source error to surface")
		}
	}
	if calls != 2 {
		t.Fatalf("source called %d times, want 2 (a failure must not be cached)", calls)
	}
}

func TestCacheIsSafeForConcurrentCards(t *testing.T) {
	gameID := uuid.New()
	cache := NewCache(func(context.Context) (map[uuid.UUID]Counts, error) {
		return map[uuid.UUID]Counts{gameID: {Playing: 1}}, nil
	}, time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cache.For(context.Background(), gameID); err != nil {
				t.Errorf("For: %v", err)
			}
		}()
	}
	wg.Wait()
}
