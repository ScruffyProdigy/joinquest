package gameclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestEligibilityCacheHitsWithinTTL(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"modes": {"legendary": {"accessible": true}}}`))
	}))
	defer srv.Close()

	cache := NewEligibilityCache(NewClient(), 50*time.Millisecond)

	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1"); err != nil {
		t.Fatalf("first Get failed: %v", err)
	}
	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1"); err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected 1 upstream call (cache hit on second Get), got %d", got)
	}
}

func TestEligibilityCacheExpiresAfterTTL(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"modes": {}}`))
	}))
	defer srv.Close()

	cache := NewEligibilityCache(NewClient(), 10*time.Millisecond)

	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1"); err != nil {
		t.Fatalf("first Get failed: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1"); err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Fatalf("expected 2 upstream calls (cache expired), got %d", got)
	}
}

func TestEligibilityCacheKeysByGameAndPlayer(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"modes": {}}`))
	}))
	defer srv.Close()

	cache := NewEligibilityCache(NewClient(), time.Second)

	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-2"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Fatalf("expected 2 upstream calls (different players), got %d", got)
	}
}

func TestEligibilityCacheStaleTimerRace(t *testing.T) {
	// This test proves that a stale AfterFunc timer cannot evict a freshly-fetched cache entry
	// that was stored by a concurrent miss. The scenario: two concurrent Get calls on the same key
	// that both miss the cache and fetch upstream, each scheduling its own AfterFunc expiry.
	// The first timer must not delete the entry if the second, fresher fetch has since overwritten it.
	//
	// We test this deterministically by:
	// 1. Call Get once (stores token=1, schedules timer for ttl)
	// 2. Simulate a second concurrent fetch by manually storing a second entry with token=2 before timer fires
	// 3. Wait for the first timer to fire
	// 4. Assert the second (fresher) entry is still present
	//
	// Without the token check in the timer callback, the first timer would unconditionally delete
	// items[key], evicting the fresh entry before its TTL window has elapsed.

	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"modes": {"legendary": {"accessible": true}}}`))
	}))
	defer srv.Close()

	cache := NewEligibilityCache(NewClient(), 20*time.Millisecond)

	// First Get: fetches and stores with token=1, schedules timer.
	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1"); err != nil {
		t.Fatalf("first Get failed: %v", err)
	}

	// Verify token=1 is stored.
	cache.mu.Lock()
	if entry, ok := cache.items["game-1|player-1"]; !ok {
		cache.mu.Unlock()
		t.Fatalf("expected entry in cache after first Get")
	} else if entry.token != 1 {
		cache.mu.Unlock()
		t.Fatalf("expected token=1 after first Get, got %d", entry.token)
	}
	cache.mu.Unlock()

	// Simulate second concurrent fetch by directly storing a new entry with token=2.
	// In the real race, this would be done by a second concurrent Get() call that wins the fetch,
	// but we do it here deterministically to avoid flaky timing.
	cache.mu.Lock()
	cache.nextTok++
	cache.items["game-1|player-1"] = cacheEntry{
		data:  map[string]ModeEligibility{"legendary": {Accessible: true}},
		token: cache.nextTok,
	}
	newToken := cache.nextTok
	cache.mu.Unlock()

	// Now let the first timer fire (20ms).
	time.Sleep(40 * time.Millisecond)

	// Assert the entry is still present (not evicted by the first timer).
	cache.mu.Lock()
	if entry, ok := cache.items["game-1|player-1"]; !ok {
		cache.mu.Unlock()
		t.Fatalf("expected entry to still be in cache after first timer fired; stale timer evicted it")
	} else if entry.token != newToken {
		cache.mu.Unlock()
		t.Fatalf("expected token=%d (the fresh one), got %d", newToken, entry.token)
	}
	cache.mu.Unlock()
}
