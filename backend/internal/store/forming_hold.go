package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// How long a formed-but-unfired table waits for a player who is not looking.
//
// These are starting values to be tuned on real data, not measured ones. They are
// deliberately short: the table is already complete, so every second spent waiting
// is a second the players who did show up spend still queuing.
//
// The split is self-balancing against queue depth rather than arbitrary. A busy
// queue has a replacement available immediately, so the chair frees at the soft
// value and nobody waits; a thin queue has none, which is exactly when the absent
// player is worth waiting the full ceiling for.
const (
	// HoldUnreachableFloor applies when nothing can tell the player to come back.
	// Waiting longer than this only taxes the players who stayed.
	HoldUnreachableFloor = 15 * time.Second

	// HoldReachableSoft is the point at which a reachable player's chair is freed
	// as soon as the waiting pool can fill it.
	HoldReachableSoft = 45 * time.Second

	// HoldReachableCeiling is the longest a chair is ever held. Sized for a phone
	// in a pocket -- push delivery, noticing it, unlocking, tapping, reconnecting --
	// not for someone sitting at a desk with the client in front of them.
	HoldReachableCeiling = 2 * time.Minute
)

// holdWindowFor returns how long this player's chair may be held.
//
// Only the floor is reachable today. The longer windows are earned by being
// push-reachable, and the reachability predicate that decides it lands with
// JQ-198; until then every hold is treated as unreachable, which is the
// conservative direction — a shorter hold costs the absent player their chair,
// while a longer one costs everyone else their time.
func holdWindowFor(pushReachable bool) time.Duration {
	if pushReachable {
		return HoldReachableCeiling
	}
	return HoldUnreachableFloor
}

// advanceHoldTx records that this player's chair is being held and reports whether
// the window has run out.
//
// Starting a fresh window when the held player changes is the point: the window
// belongs to the absence being waited out, not to the match, so a player who
// wanders off just as another returns is not charged for the time already spent.
func advanceHoldTx(ctx context.Context, tx *sql.Tx, formingMatchID, userID uuid.UUID, window time.Duration) (expired bool, err error) {
	var startedAt time.Time
	err = tx.QueryRowContext(ctx, `
		UPDATE forming_matches
		SET hold_started_at = CASE
		        WHEN held_user_id IS DISTINCT FROM $2 OR hold_started_at IS NULL THEN NOW()
		        ELSE hold_started_at
		    END,
		    held_user_id = $2
		WHERE id = $1
		RETURNING hold_started_at
	`, formingMatchID, userID).Scan(&startedAt)
	if err != nil {
		return false, fmt.Errorf("advance forming hold: %w", err)
	}
	return time.Since(startedAt) >= window, nil
}

// clearHoldTx forgets any running window. Called both when the held player comes
// back and when their chair is given up, so a later absence cannot inherit a stamp
// and expire the instant it starts.
func clearHoldTx(ctx context.Context, tx *sql.Tx, formingMatchID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE forming_matches
		SET hold_started_at = NULL, held_user_id = NULL
		WHERE id = $1 AND (hold_started_at IS NOT NULL OR held_user_id IS NOT NULL)
	`, formingMatchID)
	if err != nil {
		return fmt.Errorf("clear forming hold: %w", err)
	}
	return nil
}

// heldUserIDTx returns whose absence the running window is waiting out, or nil when
// no window is running.
func heldUserIDTx(ctx context.Context, tx *sql.Tx, formingMatchID uuid.UUID) (*uuid.UUID, error) {
	var userID *uuid.UUID
	err := tx.QueryRowContext(ctx, `
		SELECT held_user_id FROM forming_matches WHERE id = $1
	`, formingMatchID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("held user id: %w", err)
	}
	return userID, nil
}
