package ratingworker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/store"
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

// fakeSweeper reports a fixed set of modes as needing replay, the way a
// store-backed Sweeper would after ListModesNeedingReplay found inputs newer
// than their last replay.
type fakeSweeper struct {
	mu    sync.Mutex
	modes []store.RatedMode
	err   error
	calls int
}

func (f *fakeSweeper) ListModesNeedingReplay(ctx context.Context) ([]store.RatedMode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.modes, f.err
}

func (f *fakeSweeper) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// replayerFunc adapts a plain function to the Replayer interface, so a test
// can plug in custom timing or bookkeeping without a dedicated struct.
type replayerFunc func(ctx context.Context, gameID, modeKey string) (rating.Report, error)

func (f replayerFunc) ReplayMode(ctx context.Context, gameID, modeKey string) (rating.Report, error) {
	return f(ctx, gameID, modeKey)
}

// TestSweepOnceSchedulesModesFromSweeper proves the sweep arm's only effect
// is to mark modes dirty — the replay itself still happens through the
// normal DrainNow path, so a sweep and a freshly reported result coalesce
// into the same drain rather than racing each other.
func TestSweepOnceSchedulesModesFromSweeper(t *testing.T) {
	fake := &fakeReplayer{}
	w := New(func() Replayer { return fake }, time.Hour)

	game := uuid.New()
	sweeper := &fakeSweeper{modes: []store.RatedMode{{GameID: game, ModeKey: "arena"}}}
	w.SetSweeper(sweeper, time.Hour)

	w.sweepOnce(context.Background())
	if len(fake.calls) != 0 {
		t.Fatalf("sweepOnce replayed directly (%v); it must only Schedule and leave the replay to DrainNow", fake.calls)
	}

	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("DrainNow: %v", err)
	}
	want := game.String() + "/arena"
	if len(fake.calls) != 1 || fake.calls[0] != want {
		t.Fatalf("replay calls = %v, want [%s]", fake.calls, want)
	}
}

// TestSweepOnceSurvivesSweeperError proves a sweep that fails to query the
// store logs and returns rather than panicking or wedging the worker — the
// same shape as DrainNow's own per-mode error handling.
func TestSweepOnceSurvivesSweeperError(t *testing.T) {
	fake := &fakeReplayer{}
	w := New(func() Replayer { return fake }, time.Hour)
	sweeper := &fakeSweeper{err: errors.New("boom")}
	w.SetSweeper(sweeper, time.Hour)

	w.sweepOnce(context.Background())

	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("DrainNow: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("replayed %v after a failed sweep; nothing should have been scheduled", fake.calls)
	}
}

// TestStartSweepsBeforeEnteringTheLoop proves Start runs the sweep once
// up front rather than waiting for the first sweep tick, so a process that
// restarted after losing scheduled work repairs itself immediately instead
// of serving stale ratings for a whole sweepEvery interval.
func TestStartSweepsBeforeEnteringTheLoop(t *testing.T) {
	fake := &fakeReplayer{}
	w := New(func() Replayer { return fake }, time.Hour)

	game := uuid.New()
	sweeper := &fakeSweeper{modes: []store.RatedMode{{GameID: game, ModeKey: "arena"}}}
	// Both intervals are an hour: if the pre-loop sweep did not happen, no
	// tick fires before the context times out below and calls would stay 0.
	w.SetSweeper(sweeper, time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	w.Start(ctx)

	if sweeper.callCount() != 1 {
		t.Fatalf("sweeper called %d times, want 1 — Start must sweep once before its ticker loop begins", sweeper.callCount())
	}
	if err := w.DrainNow(context.Background()); err != nil {
		t.Fatalf("DrainNow: %v", err)
	}
	want := game.String() + "/arena"
	if len(fake.calls) != 1 || fake.calls[0] != want {
		t.Fatalf("replay calls = %v, want [%s] — the pre-loop sweep's schedule must survive to the next drain", fake.calls, want)
	}
}

// TestStartWithoutSweeperNeverSweeps proves the sweep arm stays inert when
// no sweeper is configured — SetSweeper is opt-in, and a worker built with
// New alone must keep its old behavior of replaying only what Schedule is
// told about.
func TestStartWithoutSweeperNeverSweeps(t *testing.T) {
	fake := &fakeReplayer{}
	w := New(func() Replayer { return fake }, time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	w.Start(ctx)

	if len(fake.calls) != 0 {
		t.Fatalf("replayed %v with no sweeper configured and nothing scheduled; want no calls", fake.calls)
	}
}

// TestStartNeverRunsConcurrentDrains guards the invariant the sweep arm must
// not break: SaveRatings clears and reinserts a mode's rows in one
// transaction, so two replays for the same mode racing each other can commit
// out of order and leave last_rated_at older than what is actually cached
// (JQ-140 task 5 review). Start's select loop only ever runs one case at a
// time, and the sweep arm never calls DrainNow itself — it only calls
// Schedule, which just marks the dirty set — so DrainNow (the only path that
// replays) can never have two invocations in flight for one worker. This
// drives both arms as fast as possible with a replay that reports how many
// concurrent calls it saw, to catch a regression that adds a second
// concurrent drain path.
func TestStartNeverRunsConcurrentDrains(t *testing.T) {
	var inFlight int32
	var maxInFlight int32
	fake := replayerFunc(func(ctx context.Context, gameID, modeKey string) (rating.Report, error) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&maxInFlight)
			if n <= old {
				break
			}
			if atomic.CompareAndSwapInt32(&maxInFlight, old, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		return rating.Report{}, nil
	})

	w := New(func() Replayer { return fake }, 2*time.Millisecond)
	sweeper := &fakeSweeper{modes: []store.RatedMode{
		{GameID: uuid.New(), ModeKey: "arena"},
		{GameID: uuid.New(), ModeKey: "duel"},
	}}
	w.SetSweeper(sweeper, 2*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	w.Start(ctx)

	if got := atomic.LoadInt32(&maxInFlight); got > 1 {
		t.Fatalf("max concurrent replay calls observed = %d, want at most 1 — the sweep and tick arms must never drain concurrently", got)
	}
}
