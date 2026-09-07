package store

import (
	"context"
	"fmt"
	"time"
)

// DefaultStaleMatchedQueueAge is how long a `matched` queue row may sit without a
// live session before the sweep treats it as orphaned rather than provisioning.
//
// A matched row is legitimately transient while a session is being created and the
// handoff completes, so this doubles as the match-proposal deadline we never wrote
// down: past it, the default outcome of silence is dissolution, not a stuck player.
const DefaultStaleMatchedQueueAge = 5 * time.Minute

// staleMatchedQueuePredicate matches `matched` rows with no active, unfinished
// participation in a session for the same game and catalog queue.
//
// It deliberately mirrors reconcileStaleMatchedQueuesForUserTx so the per-user
// lazy heal and the global sweep cannot drift apart on what "stale" means. The
// age guard ($1, a Postgres interval) is the only thing the sweep adds, because
// it runs fleet-wide against matches that may still be forming.
//
// matched_at falls back to joined_at: legacy rows can carry a NULL matched_at,
// and those are exactly the orphans worth catching rather than skipping.
const staleMatchedQueuePredicate = `
	gq.status = 'matched'
	  AND gq.mode_queue_id IS NOT NULL
	  AND COALESCE(gq.matched_at, gq.joined_at) < NOW() - $1::interval
	  AND NOT EXISTS (
	    SELECT 1
	    FROM game_session_participants gsp
	    INNER JOIN game_sessions gs
	      ON gs.id = gsp.session_id
	     AND gs.status = 'active'
	     AND gs.game_id = gq.game_id
	     AND gs.mode_queue_id = gq.mode_queue_id
	    WHERE gsp.user_id = gq.user_id
	      AND gsp.left_at IS NULL
	      AND gsp.finished_at IS NULL
	  )`

// SweepResult reports what one pass of the stale matched queue sweep saw and did.
type SweepResult struct {
	// StaleBefore is how many stale rows the sweep found.
	StaleBefore int
	// Cancelled is how many rows it actually transitioned to 'cancelled'.
	Cancelled int
	// StaleAfter is how many stale rows remain. Anything above zero means rows
	// resisted the sweep and a human should look.
	StaleAfter int
}

// pgInterval renders a duration as a Postgres interval literal.
func pgInterval(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf("%d seconds", int64(d.Seconds()))
}

// CountStaleMatchedQueues returns how many matched queue rows across all users are
// older than olderThan with no live session behind them. It writes nothing, so it is
// safe to call from a health check or an alert probe.
func (s *Store) CountStaleMatchedQueues(ctx context.Context, olderThan time.Duration) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM game_queues gq WHERE `+staleMatchedQueuePredicate,
		pgInterval(olderThan),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count stale matched queues: %w", err)
	}
	return count, nil
}

// SweepStaleMatchedQueues cancels stale matched queue rows across all users, not just
// the one making a request. It is the scheduled counterpart to the per-user heal that
// runs inside GetUserActiveIntent.
//
// The sweep is idempotent: a second pass over already-cancelled rows matches nothing,
// because the predicate only selects rows still in 'matched'. It is safe to run while
// players are queueing and playing, because a row with a live session never matches the
// predicate and a row younger than olderThan is left to finish provisioning.
func (s *Store) SweepStaleMatchedQueues(ctx context.Context, olderThan time.Duration) (SweepResult, error) {
	var result SweepResult
	interval := pgInterval(olderThan)

	before, err := s.CountStaleMatchedQueues(ctx, olderThan)
	if err != nil {
		return result, err
	}
	result.StaleBefore = before

	// Counting and cancelling in one transaction keeps the reported numbers
	// consistent with the rows actually touched.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin stale matched queue sweep: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		UPDATE game_queues gq
		SET status = 'cancelled'
		WHERE `+staleMatchedQueuePredicate, interval)
	if err != nil {
		return result, fmt.Errorf("sweep stale matched queues: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return result, fmt.Errorf("sweep stale matched queues rows affected: %w", err)
	}
	result.Cancelled = int(affected)

	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit stale matched queue sweep: %w", err)
	}

	after, err := s.CountStaleMatchedQueues(ctx, olderThan)
	if err != nil {
		return result, err
	}
	result.StaleAfter = after

	return result, nil
}
