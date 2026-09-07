package gameclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newCountingRosterServer(t *testing.T, hits *int64, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(hits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"groups":[{"key":"helpers","choices":[{"id":"ferrus","label":"Ferrus"}]}]}`))
	}))
}

func TestQueueOptionsCacheServesSecondReadWithoutRefetching(t *testing.T) {
	var hits int64
	srv := newCountingRosterServer(t, &hits, http.StatusOK)
	defer srv.Close()
	cache := NewQueueOptionsCache(NewClient(), time.Minute)

	for i := 0; i < 3; i++ {
		if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1", "duel-helpers", "helpers"); err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
	}

	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Fatalf("game server hits = %d, want 1", got)
	}
}

func TestQueueOptionsCacheKeepsModesApart(t *testing.T) {
	var hits int64
	srv := newCountingRosterServer(t, &hits, http.StatusOK)
	defer srv.Close()
	cache := NewQueueOptionsCache(NewClient(), time.Minute)

	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1", "duel-helpers", "helpers"); err != nil {
		t.Fatalf("Get duel-helpers: %v", err)
	}
	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1", "duel-decks", "decks"); err != nil {
		t.Fatalf("Get duel-decks: %v", err)
	}

	if got := atomic.LoadInt64(&hits); got != 2 {
		t.Fatalf("game server hits = %d, want 2 — a second mode must not read the first mode's roster", got)
	}
}

func TestQueueOptionsCacheDoesNotCacheFailures(t *testing.T) {
	var hits int64
	srv := newCountingRosterServer(t, &hits, http.StatusInternalServerError)
	defer srv.Close()
	cache := NewQueueOptionsCache(NewClient(), time.Minute)

	for i := 0; i < 2; i++ {
		if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1", "duel-helpers", "helpers"); err == nil {
			t.Fatalf("Get %d = nil error, want the 500 surfaced", i)
		}
	}

	if got := atomic.LoadInt64(&hits); got != 2 {
		t.Fatalf("game server hits = %d, want 2 — a failed roster must be retried, not remembered", got)
	}
}

func TestQueueOptionsCacheExpires(t *testing.T) {
	var hits int64
	srv := newCountingRosterServer(t, &hits, http.StatusOK)
	defer srv.Close()
	cache := NewQueueOptionsCache(NewClient(), 20*time.Millisecond)

	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1", "duel-helpers", "helpers"); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1", "duel-helpers", "helpers"); err != nil {
		t.Fatalf("second Get: %v", err)
	}

	if got := atomic.LoadInt64(&hits); got != 2 {
		t.Fatalf("game server hits = %d, want 2 after the entry expired", got)
	}
}
