package ratingworker

import (
	"context"
	"errors"
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

// flakyReplayer fails ReplayMode for whichever keys are listed in failFor,
// letting a test flip a key from failing to succeeding between drains.
type flakyReplayer struct {
	mu      sync.Mutex
	calls   []string
	failFor map[string]bool
}

func (f *flakyReplayer) ReplayMode(ctx context.Context, gameID, modeKey string) (rating.Report, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := gameID + "/" + modeKey
	f.calls = append(f.calls, key)
	if f.failFor[key] {
		return rating.Report{}, errors.New("boom")
	}
	return rating.Report{}, nil
}

// TestDrainRequeuesModeWhoseReplayFails proves a failed replay is not
// dropped: DrainNow must put the mode back into the dirty set so the next
// tick retries it, rather than only logging the error and moving on. Without
// this, a correction whose replay fails would never be retried, since a
// correction leaves max(rated_at) unchanged and so never looks dirty to a
// sweep either (see the package comment).
func TestDrainRequeuesModeWhoseReplayFails(t *testing.T) {
	game := uuid.New()
	key := game.String() + "/arena"
	fake := &flakyReplayer{failFor: map[string]bool{key: true}}
	w := New(func() Replayer { return fake }, time.Hour)

	w.Schedule(game, "arena")
	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("DrainNow: %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("replayed %d times (%v), want 1 attempt", len(fake.calls), fake.calls)
	}

	// No new Schedule call here: the retry must come from the failed replay
	// re-queueing itself, not from a fresh caller noticing the mode again.
	fake.mu.Lock()
	fake.failFor[key] = false
	fake.mu.Unlock()
	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("second DrainNow: %v", err)
	}
	if len(fake.calls) != 2 {
		t.Fatalf("replayed %d times after second drain (%v), want 2 — a failed replay must be retried, not dropped", len(fake.calls), fake.calls)
	}

	// The retry succeeded, so a third drain with nothing newly scheduled
	// must not replay again.
	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("third DrainNow: %v", err)
	}
	if len(fake.calls) != 2 {
		t.Fatalf("replayed %d times after third drain (%v), want still 2 — a succeeded retry must not stay queued", len(fake.calls), fake.calls)
	}
}
