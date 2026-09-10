// Package ratingworker recomputes a mode's ratings after its match results
// change.
//
// Ratings are applied by replaying a mode's whole input log, not by folding
// each match in as it lands. rating_match_inputs is the source of truth and
// player_ratings is a cache of a replay over it, so a replay is idempotent by
// construction — which is what makes a re-reported result harmless — and it
// is the only way a *corrected* result can take effect, since a correction
// rewrites a row that sits in the middle of history (see appendRatingInput's
// note on preserving rated_at).
//
// A replay costs O(history) for that game and mode, so it runs here rather
// than inside the transaction that records the result: a game server's
// callback must not get slower as the mode accumulates matches. Work is
// coalesced per (game, mode) — ten matches finishing in one tick produce one
// replay, not ten, and the tenth replay would have subsumed the other nine
// anyway.
//
// Losing scheduled work is survivable for a new match: the log is already
// committed, so the next match in that mode replays everything, and
// ListModesNeedingReplay sweeps the case where no next match arrives. A
// failed replay itself is retried — DrainNow re-queues a mode whose
// ReplayMode call errors, so the next tick tries again.
//
// A *corrected* result is a narrower case. appendRatingInput deliberately
// does not refresh rated_at when a match is re-reported (a correction is a
// restatement of a match that already happened, not a new event — see its
// comment for why bumping rated_at would reorder replay history). So a
// correction never changes max(rated_at) for its mode, and a replay that
// already ran for that correction leaves last_rated_at unchanged too — the
// two already matched before the correction landed. If the correction's
// replay is scheduled but then lost (process crash before the next tick) or
// exhausts its retries, the mode looks clean to a sweep that compares
// max-input against last-rated forever, and the stale ratings persist until
// an unrelated new match happens to land in that mode. This is a known,
// accepted limit of the replay-by-log design, not something this package
// closes.
package ratingworker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// Replayer is the narrow port this worker needs; *rating.Replayer satisfies it.
type Replayer interface {
	ReplayMode(ctx context.Context, gameID, modeKey string) (rating.Report, error)
}

// Sweeper reports modes whose ratings are behind their inputs.
type Sweeper interface {
	ListModesNeedingReplay(ctx context.Context) ([]store.RatedMode, error)
}

type modeKey struct {
	gameID  uuid.UUID
	modeKey string
}

// Worker replays dirty modes on a tick.
type Worker struct {
	newReplayer func() Replayer
	tickEvery   time.Duration

	sweeper    Sweeper
	sweepEvery time.Duration

	mu    sync.Mutex
	dirty map[modeKey]struct{}
}

// New builds a Worker. newReplayer is called once per replay run rather than
// held, because the store's rating adapter carries per-run state and must not
// be shared across concurrent replays (JQ-241).
func New(newReplayer func() Replayer, tickEvery time.Duration) *Worker {
	return &Worker{
		newReplayer: newReplayer,
		tickEvery:   tickEvery,
		dirty:       make(map[modeKey]struct{}),
	}
}

// Schedule marks a mode as needing a replay. Safe to call from any goroutine,
// and cheap enough to call on every reported result.
func (w *Worker) Schedule(gameID uuid.UUID, mode string) {
	if mode == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.dirty[modeKey{gameID: gameID, modeKey: mode}] = struct{}{}
}

// SetSweeper enables the periodic catch-up sweep. Without one, the worker
// replays only what it is told about.
func (w *Worker) SetSweeper(s Sweeper, every time.Duration) {
	w.sweeper = s
	w.sweepEvery = every
}

// Start runs the tick loop until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.tickEvery)
	defer ticker.Stop()

	var sweep <-chan time.Time
	if w.sweeper != nil {
		// Sweep once before entering the loop, so a process that restarted
		// after losing scheduled work repairs itself immediately rather than
		// serving stale ratings for a whole interval.
		w.sweepOnce(ctx)
		sweepTicker := time.NewTicker(w.sweepEvery)
		defer sweepTicker.Stop()
		sweep = sweepTicker.C
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.DrainNow(ctx); err != nil {
				log.Printf("ratingworker: drain: %v", err)
			}
		case <-sweep:
			w.sweepOnce(ctx)
		}
	}
}

// sweepOnce marks every mode whose ratings lag its inputs and leaves the next
// drain to do the work, so a sweep and a freshly reported result coalesce
// into one replay rather than racing each other. This also keeps DrainNow
// the sole caller of a replay: sweepOnce only ever touches the dirty set
// through Schedule, so two replays for the same mode can never be in flight
// at once (see the package comment on DrainNow being the single drain path).
func (w *Worker) sweepOnce(ctx context.Context) {
	modes, err := w.sweeper.ListModesNeedingReplay(ctx)
	if err != nil {
		log.Printf("ratingworker: sweep: %v", err)
		return
	}
	for _, m := range modes {
		w.Schedule(m.GameID, m.ModeKey)
	}
}

// DrainNow replays every mode currently marked dirty and returns once they
// are done. Exported so tests and the sweep can force a pass without waiting
// for a tick.
func (w *Worker) DrainNow(ctx context.Context) error {
	w.mu.Lock()
	pending := make([]modeKey, 0, len(w.dirty))
	for k := range w.dirty {
		pending = append(pending, k)
	}
	w.dirty = make(map[modeKey]struct{})
	w.mu.Unlock()

	var failed []modeKey
	for _, k := range pending {
		replayer := w.newReplayer()
		if _, err := replayer.ReplayMode(ctx, k.gameID.String(), k.modeKey); err != nil {
			// Log and continue: one mode failing to replay must not stop the
			// others. Re-queue it below so the next tick retries it — the
			// input log is intact either way, but a failed replay must not
			// be dropped on the floor, since a corrected result's replay
			// failing here is not self-healing the way a new match's would
			// be (see the package comment).
			log.Printf("ratingworker: replay %s/%s: %v", k.gameID, k.modeKey, err)
			failed = append(failed, k)
		}
	}

	if len(failed) > 0 {
		w.mu.Lock()
		for _, k := range failed {
			w.dirty[k] = struct{}{}
		}
		w.mu.Unlock()
	}
	return nil
}
