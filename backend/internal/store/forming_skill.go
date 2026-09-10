package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/dispersion"
	"github.com/scruffyprodigy/joinquest/internal/queuewait"
	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// LambdaWindow is how far back arrivals are counted when measuring a line's
// rate. Long enough for a stable estimate on a thin queue, short enough to
// track time-of-day swings.
//
// JQ-142's design asks for this exponentially weighted, so the estimate reacts
// without a cliff when the window edge passes. This is a flat window because
// that is what JQ-244's RecentQueueFlow measures; the shape of the decision is
// the same either way, and weighting is a refinement to make in queuewait
// rather than a second rate measured here.
const LambdaWindow = 15 * time.Minute

// skillFireDecisionTx answers whether a full forming map should fire now.
//
// It is the second term on the fire condition. Today a lobby fires when the map
// is full; with this it fires when the map is full AND this says so. Everything
// it needs is re-derived from durable state on every tick -- the mode's switch,
// the line's arrival rate, the oldest waiter's age, the ratings of whoever is
// currently seated -- so there is no deferral to persist and nothing for a
// second worker pod to lose. A decision not to fire is simply a tick that did
// not fire.
func (s *Store) skillFireDecisionTx(
	ctx context.Context,
	tx *sql.Tx,
	joinCtx *ModeQueueJoinContext,
	fm *FormingMatch,
	waiting []QueueEntry,
	now time.Time,
) (dispersion.Decision, error) {
	if !joinCtx.Mode.SkillMatchingEnabled {
		// Short-circuited before any rating is read or any rate is measured. A
		// mode that has opted out should not pay for the decision either.
		return dispersion.Decide(dispersion.Input{SkillMatchingEnabled: false}), nil
	}

	assignments, err := s.ListFormingAssignmentsTx(ctx, tx, fm.ID)
	if err != nil {
		return dispersion.Decision{}, err
	}
	seated := make([]uuid.UUID, 0, len(assignments))
	for _, a := range assignments {
		if a.UserID != nil {
			seated = append(seated, *a.UserID)
		}
	}

	ratings, err := s.GetPlayerRatings(ctx, joinCtx.Game.ID, joinCtx.Mode.ModeKey, seated)
	if err != nil {
		return dispersion.Decision{}, err
	}
	mus := make([]float64, 0, len(seated))
	for _, id := range seated {
		if r, ok := ratings[id]; ok {
			mus = append(mus, r.Mu)
			continue
		}
		// Unrated players are placed at the documented cold-start prior --
		// never excluded from matchmaking, and never quietly dropped from the
		// dispersion so that a lobby full of them scores artificially well.
		// The prior is roughly the population centre, so this is honest rather
		// than a fudge: a lobby of entirely unrated players scores zero
		// dispersion, which is vacuous and also the best estimate available.
		mus = append(mus, rating.UnratedMu)
	}

	lambda, err := s.scarcestArrivalRate(ctx, joinCtx.ModeQueue.ID, now)
	if err != nil {
		return dispersion.Decision{}, err
	}

	return dispersion.Decide(dispersion.Input{
		SkillMatchingEnabled: true,
		Lambda:               lambda,
		Seats:                len(assignments),
		OldestWait:           oldestWait(waiting, now),
		Spread:               dispersion.Of(mus),
	}), nil
}

// oldestWait is how long the longest-waiting player on this queue has been
// waiting.
//
// The oldest rather than the newest, and the reason is perceptual rather than
// arithmetic. A hold costs the player who completes the lobby just as many
// seconds as the one already waiting, but it does not cost them the same: the
// newcomer never saw the queue's state when they joined, so a three-second
// match is indistinguishable from an instant one, while the player ninety
// seconds in feels every additional second. Anchoring the budget here spends
// the wait on the player who cannot perceive it.
//
// It is also what makes the band widen without a second schedule: as this grows
// the budget shrinks, and at expiry the dispersion constraint is dropped
// entirely.
func oldestWait(waiting []QueueEntry, now time.Time) time.Duration {
	longest := time.Duration(0)
	for _, entry := range waiting {
		if age := now.Sub(entry.JoinedAt); age > longest {
			longest = age
		}
	}
	return longest
}

// scarcestArrivalRate is lambda for the slowest line in this queue.
//
// The slowest rather than the total, because a lobby cannot complete faster
// than its scarcest bucket fills. A composition mode whose damage queue is
// flooded and whose support queue is empty has no candidates to choose between,
// however busy it looks in aggregate -- and holding on that aggregate would be
// paying a wait for a choice that does not exist.
func (s *Store) scarcestArrivalRate(ctx context.Context, modeQueueID uuid.UUID, now time.Time) (float64, error) {
	flows, err := s.RecentQueueFlow(ctx, queuewait.FlowQuery{
		ModeQueueIDs: []uuid.UUID{modeQueueID},
		Since:        now.Add(-LambdaWindow),
		Now:          now,
	})
	if err != nil {
		return 0, err
	}

	scarcest := 0.0
	first := true
	for key, flow := range flows {
		if key.ModeQueueID != modeQueueID {
			continue
		}
		rate := flow.ArrivalRate()
		if first || rate < scarcest {
			scarcest = rate
			first = false
		}
	}
	return scarcest, nil
}
