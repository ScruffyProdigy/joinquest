package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DefaultQueueDisconnectGrace is how long a waiting player's socket may stay down
// before the queue treats the disconnect as a departure.
//
// 90s is chosen against mobile backgrounding behaviour rather than measured: glancing
// at a notification, a tunnel, a lift, a closed laptop lid are all ordinary and all
// shorter than this. Tune it on how often the window actually expires versus how
// often players reconnect inside it — a high expiry rate on short queues would say it
// is still too aggressive.
//
// This is NOT the seat-hold window for an already-formed match, which answers a
// different question (how long a formed match waits for a player who is not there)
// and carries its own number. Do not borrow one for the other.
const DefaultQueueDisconnectGrace = 90 * time.Second

// EvictionResult reports whether an expiry actually removed a player, and what the
// caller needs in order to publish the same queue update a deliberate leave publishes.
type EvictionResult struct {
	Acted       bool
	ModeQueueID uuid.UUID
	GameID      uuid.UUID
	// QueuedCount is the count AFTER the removal, which is what the remaining
	// players' queued counts should show.
	QueuedCount int
}

// EvictDisconnectedWaitingEntry cancels a user's waiting queue row if and only if
// their socket is still down and the disconnect stamp still matches the one the
// caller armed its timer on.
//
// Every safety condition is a predicate on the UPDATE rather than a preceding SELECT.
// A read-then-write here would be check-then-act: the row can flip waiting -> matched
// in the gap, and the advisory lock the forming path takes does not exclude a plain
// SELECT, because a plain SELECT never contends for it. Zero rows affected means
// somebody got there first — a reconnect, a deliberate leave, or a match forming —
// which is exactly the outcome we want.
//
// It deliberately does NOT reuse LeaveModeQueue. That helper also calls
// cancelUserMatchedModeQueue, whose UPDATE is unconditionally scoped to
// status = 'matched'. Reusing it would cancel a held seat through the *action* even
// though the guard above never let a matched row through the *check* — the failure
// would read as correct in review. LeaveModeQueue stays right for a deliberate leave,
// where the player does want out of both states.
func (s *Store) EvictDisconnectedWaitingEntry(ctx context.Context, userID uuid.UUID, stamp time.Time) (EvictionResult, error) {
	var out EvictionResult

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("begin disconnect eviction: %w", err)
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		UPDATE game_queues gq
		SET status = 'cancelled'
		FROM user_presence up
		WHERE gq.user_id = $1
		  AND gq.status = 'waiting'
		  AND gq.mode_queue_id IS NOT NULL
		  AND up.user_id = gq.user_id
		  AND up.connection_count = 0
		  AND up.disconnected_at = $2
		RETURNING gq.mode_queue_id, gq.game_id
	`, userID, stamp).Scan(&out.ModeQueueID, &out.GameID)
	if errors.Is(err, sql.ErrNoRows) {
		return EvictionResult{}, nil
	}
	if err != nil {
		return EvictionResult{}, fmt.Errorf("evict disconnected waiting entry: %w", err)
	}
	out.Acted = true

	if err := tx.QueryRowContext(ctx, `
		SELECT count(*) FROM game_queues WHERE mode_queue_id = $1 AND status = 'waiting'
	`, out.ModeQueueID).Scan(&out.QueuedCount); err != nil {
		return EvictionResult{}, fmt.Errorf("count waiting after eviction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return EvictionResult{}, fmt.Errorf("commit disconnect eviction: %w", err)
	}

	// Mirrors leaveModeQueueWaiting: a party left behind by the removed player is
	// reconciled the same way a deliberate leave reconciles it. Deliberately after
	// the commit, so a party problem cannot roll back an eviction that already
	// decided correctly.
	if err := s.reconcileStalePartiesForUser(ctx, userID); err != nil {
		return out, fmt.Errorf("reconcile parties after eviction: %w", err)
	}
	return out, nil
}
