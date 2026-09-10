package ratingworker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

type fakeReplayer struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeReplayer) ReplayMode(ctx context.Context, gameID, modeKey string) (rating.Report, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, gameID+"/"+modeKey)
	return rating.Report{}, nil
}

func TestDrainCoalescesRepeatedSchedules(t *testing.T) {
	fake := &fakeReplayer{}
	w := New(func() Replayer { return fake }, time.Hour)

	game := uuid.New()
	w.Schedule(game, "arena")
	w.Schedule(game, "arena")
	w.Schedule(game, "duel")

	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("DrainNow: %v", err)
	}

	if len(fake.calls) != 2 {
		t.Fatalf("replayed %d times (%v), want 2 — arena twice should coalesce", len(fake.calls), fake.calls)
	}
}

func TestDrainClearsTheQueue(t *testing.T) {
	fake := &fakeReplayer{}
	w := New(func() Replayer { return fake }, time.Hour)

	w.Schedule(uuid.New(), "arena")
	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("DrainNow: %v", err)
	}
	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("second DrainNow: %v", err)
	}

	if len(fake.calls) != 1 {
		t.Fatalf("replayed %d times, want 1 — a drained mode should not replay again", len(fake.calls))
	}
}

func TestNewReplayerIsCalledPerRun(t *testing.T) {
	built := 0
	w := New(func() Replayer { built++; return &fakeReplayer{} }, time.Hour)

	w.Schedule(uuid.New(), "arena")
	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("DrainNow: %v", err)
	}
	w.Schedule(uuid.New(), "duel")
	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("DrainNow: %v", err)
	}

	if built < 2 {
		t.Fatalf("replayer factory called %d times, want one per run — a shared adapter cross-contaminates (JQ-241)", built)
	}
}
