package ratingworker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// TestReplayUsesTheModesOwnEngine is what makes a per-mode beta take effect at
// all: two modes replayed in the same drain must be rated by their own
// engines, not by whichever one the process happened to build at startup.
func TestReplayUsesTheModesOwnEngine(t *testing.T) {
	game := uuid.New()

	var mu sync.Mutex
	usedByMode := map[string]string{}

	w := New(
		func(_ context.Context, _ uuid.UUID, modeKey string) (rating.Engine, error) {
			return stubEngine{id: "engine-for-" + modeKey}, nil
		},
		func(engine rating.Engine) Replayer {
			return replayerFunc(func(_ context.Context, _, modeKey string) (rating.Report, error) {
				mu.Lock()
				defer mu.Unlock()
				usedByMode[modeKey] = engine.ID()
				return rating.Report{}, nil
			})
		},
		testEngineID,
		time.Hour,
	)

	w.Schedule(game, "duel")
	w.Schedule(game, "party")
	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("DrainNow: %v", err)
	}

	if usedByMode["duel"] != "engine-for-duel" || usedByMode["party"] != "engine-for-party" {
		t.Errorf("modes were replayed under %v; each must use its own constants", usedByMode)
	}
}

// TestModeEngineIsResolvedPerReplay is why a newly measured beta does not need
// a restart: the lookup happens on every run, so the next replay of that mode
// picks the new constants up.
func TestModeEngineIsResolvedPerReplay(t *testing.T) {
	var lookups int
	w := New(
		func(context.Context, uuid.UUID, string) (rating.Engine, error) {
			lookups++
			return stubEngine{id: testEngineID}, nil
		},
		func(rating.Engine) Replayer { return &fakeReplayer{} },
		testEngineID,
		time.Hour,
	)

	game := uuid.New()
	for i := 0; i < 3; i++ {
		w.Schedule(game, "arena")
		if err := w.DrainNow(context.Background()); err != nil {
			t.Fatalf("DrainNow: %v", err)
		}
	}

	if lookups != 3 {
		t.Errorf("resolved the mode's engine %d time(s) over 3 replays; it must be resolved per run", lookups)
	}
}

// TestUnresolvableEngineRequeuesRatherThanRatingUnderTheDefault covers the
// failure that would be worst if it were papered over. Rating a mode under
// constants that are not its own writes numbers on the wrong scale, stamped
// with the engine id they really were computed under — so the sweep would see
// them as correct and never repair them. Staying visibly behind is the safe
// failure.
func TestUnresolvableEngineRequeuesRatherThanRatingUnderTheDefault(t *testing.T) {
	fake := &fakeReplayer{}
	failing := true

	w := New(
		func(context.Context, uuid.UUID, string) (rating.Engine, error) {
			if failing {
				return nil, errors.New("constants unavailable")
			}
			return stubEngine{id: testEngineID}, nil
		},
		func(rating.Engine) Replayer { return fake },
		testEngineID,
		time.Hour,
	)

	game := uuid.New()
	w.Schedule(game, "arena")
	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("DrainNow: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("replayed %v despite being unable to resolve the mode's engine", fake.calls)
	}

	// Re-queued, so the next tick tries again rather than dropping the mode.
	failing = false
	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("second DrainNow: %v", err)
	}
	if len(fake.calls) != 1 {
		t.Errorf("replayed %d time(s) after the lookup recovered, want 1 — the mode was dropped rather than re-queued", len(fake.calls))
	}
}

// TestSweepPassesTheDefaultEngineID is how the sweep recognises ratings left
// behind by a constants change: a mode with no measured beta must currently be
// rated by this engine, and cached ratings carrying anything else are stale.
func TestSweepPassesTheDefaultEngineID(t *testing.T) {
	w := newTestWorker(func() Replayer { return &fakeReplayer{} }, time.Hour)

	sweeper := &fakeSweeper{modes: []store.RatedMode{}}
	w.SetSweeper(sweeper, time.Hour)
	w.sweepOnce(context.Background())

	sweeper.mu.Lock()
	defer sweeper.mu.Unlock()
	if sweeper.sawEngineID != testEngineID {
		t.Errorf("sweep asked for modes behind %q, want %q", sweeper.sawEngineID, testEngineID)
	}
}
