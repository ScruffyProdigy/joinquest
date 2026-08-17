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
