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
// A mode is replayed under its own rating constants, resolved per run (see
// ModeEngine). A newly measured beta therefore takes effect through the same
// path as a new match: the sweep sees cached ratings stamped with an engine id
// that no longer matches the mode's constants, schedules the mode, and the
// replay recomputes its whole history under the new value. Nothing continues a
// mode's ratings across a constants change, because ratings produced under two
// different betas are not on one scale.
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
// replay is scheduled but then lost (process crash before the next tick),
// the mode looks clean to a sweep that compares
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

// Sweeper reports modes whose ratings are behind the inputs or the constants
// they should have been computed from. defaultEngineID names the engine a mode
// with no measured constants of its own must be rated by, so the sweep can
// recognise ratings left behind by a constants change without a lookup per
// mode.
type Sweeper interface {
	ListModesNeedingReplay(ctx context.Context, defaultEngineID string) ([]store.RatedMode, error)
}

// ModeEngine resolves the engine one mode should be rated by: its own measured
// constants where they exist, and the default everywhere else.
//
// It is a per-mode lookup rather than a single engine held for the process
// because beta is per mode (JQ-227). A mode where skill barely predicts the
// result and one where it nearly decides the match are not the same model, and
// rating them through one engine would be the assertion this replaced.
type ModeEngine func(ctx context.Context, gameID uuid.UUID, modeKey string) (rating.Engine, error)

type modeKey struct {
	gameID  uuid.UUID
	modeKey string
}

// Worker replays dirty modes on a tick.
type Worker struct {
	modeEngine      ModeEngine
	newReplayer     func(rating.Engine) Replayer
	defaultEngineID string
	tickEvery       time.Duration

	sweeper    Sweeper
	sweepEvery time.Duration

	mu    sync.Mutex
	dirty map[modeKey]struct{}
}

// New builds a Worker.
//
// modeEngine is consulted per replay, so a mode picks up a newly measured beta
// on its next run without the process restarting. newReplayer is likewise
// called once per replay run rather than held, because the store's rating
// adapter carries per-run state and must not be shared across concurrent
// replays (JQ-241).
//
// defaultEngineID is the identity of the engine used by a mode with no
// measured constants. The sweep needs it to tell ratings computed under the
// current constants from ratings left behind by a change.
func New(modeEngine ModeEngine, newReplayer func(rating.Engine) Replayer, defaultEngineID string, tickEvery time.Duration) *Worker {
	return &Worker{
		modeEngine:      modeEngine,
		newReplayer:     newReplayer,
		defaultEngineID: defaultEngineID,
		tickEvery:       tickEvery,
		dirty:           make(map[modeKey]struct{}),
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
// at once.
func (w *Worker) sweepOnce(ctx context.Context) {
	modes, err := w.sweeper.ListModesNeedingReplay(ctx, w.defaultEngineID)
	if err != nil {
		log.Printf("ratingworker: sweep: %v", err)
		return
	}
	// A healthy system sweeps up nothing, so anything found is worth a line:
	// on the first boot after deploy every mode with inputs is behind, and
	// this is the only sign that the back-fill is running at all.
	if len(modes) > 0 {
		log.Printf("ratingworker: sweep found %d mode(s) behind their inputs or constants; scheduling a replay for each", len(modes))
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
		engine, err := w.modeEngine(ctx, k.gameID, k.modeKey)
		if err != nil {
			// Re-queue rather than fall back to the default engine. Rating a
			// mode under constants that are not its own would write ratings
			// that look fine and are on the wrong scale, and the sweep could
			// not tell them from a correct replay afterwards — the engine id
			// it stamped would be the one it was actually rated under. Better
			// to leave the mode stale and visibly behind.
			log.Printf("ratingworker: resolve engine for %s/%s: %v", k.gameID, k.modeKey, err)
			failed = append(failed, k)
			continue
		}
		replayer := w.newReplayer(engine)
		report, err := replayer.ReplayMode(ctx, k.gameID.String(), k.modeKey)
		if err != nil {
			// Log and continue: one mode failing to replay must not stop the
			// others. Re-queue it below so the next tick retries it — the
			// input log is intact either way, but a failed replay must not
			// be dropped on the floor, since a corrected result's replay
			// failing here is not self-healing the way a new match's would
			// be (see the package comment).
			log.Printf("ratingworker: replay %s/%s: %v", k.gameID, k.modeKey, err)
			failed = append(failed, k)
			continue
		}
		// One line per completed replay, so an operator can tell a mode whose
		// ratings never move from one whose replays are silently failing.
		// A breadcrumb, deliberately not per-match tracing.
		log.Printf("ratingworker: replayed %s/%s over %d match(es) under %s", k.gameID, k.modeKey, report.Matches, engine.ID())
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
