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
// Losing scheduled work is survivable: the log is already committed, so the
// next match in that mode replays everything, and ListModesNeedingReplay
// sweeps the case where no next match arrives.
package ratingworker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// Replayer is the narrow port this worker needs; *rating.Replayer satisfies it.
type Replayer interface {
	ReplayMode(ctx context.Context, gameID, modeKey string) (rating.Report, error)
}

type modeKey struct {
	gameID  uuid.UUID
	modeKey string
}

// Worker replays dirty modes on a tick.
type Worker struct {
	newReplayer func() Replayer
	tickEvery   time.Duration

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

// Start runs the tick loop until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.tickEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.DrainNow(ctx); err != nil {
				log.Printf("ratingworker: drain: %v", err)
			}
		}
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

	for _, k := range pending {
		replayer := w.newReplayer()
		if _, err := replayer.ReplayMode(ctx, k.gameID.String(), k.modeKey); err != nil {
			// Log and continue: one mode failing to replay must not stop the
			// others, and the input log is intact either way, so the next
			// match or sweep retries it.
			log.Printf("ratingworker: replay %s/%s: %v", k.gameID, k.modeKey, err)
		}
	}
	return nil
}
