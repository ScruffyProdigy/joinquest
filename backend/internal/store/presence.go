package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PresenceTransition reports what one socket edge did to a user's presence.
//
// Edge marks the transitions worth publishing: 0 -> 1 on connect and 1 -> 0 on
// disconnect. A second tab opening, or one of three sockets closing, is not an edge —
// the player's presence did not change, only their socket count.
type PresenceTransition struct {
	ConnectionCount int
	DisconnectedAt  *time.Time
	Edge            bool
}

// PresenceConnected records one more live socket for the user, clearing any
// disconnect stamp. Safe to call for a user with no presence row yet.
func (s *Store) PresenceConnected(ctx context.Context, userID uuid.UUID) (PresenceTransition, error) {
	var out PresenceTransition
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO user_presence (user_id, connection_count, disconnected_at, updated_at)
		VALUES ($1, 1, NULL, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET connection_count = user_presence.connection_count + 1,
		    disconnected_at = NULL,
		    updated_at = NOW()
		RETURNING connection_count, disconnected_at
	`, userID).Scan(&out.ConnectionCount, &out.DisconnectedAt)
	if err != nil {
		return PresenceTransition{}, fmt.Errorf("presence connected: %w", err)
	}
	out.Edge = out.ConnectionCount == 1
	return out, nil
}

// PresenceDisconnected records one fewer live socket. The stamp is written only when
// the LAST socket closes, which is what makes a second tab closing harmless.
//
// The CTE takes FOR UPDATE on the row so two sockets closing at once cannot both read
// the same count and decrement it once. COALESCE preserves an existing stamp: an
// unbalanced release must not restart a window that is already running.
func (s *Store) PresenceDisconnected(ctx context.Context, userID uuid.UUID) (PresenceTransition, error) {
	var out PresenceTransition
	var previous int
	err := s.db.QueryRowContext(ctx, `
		WITH prev AS (
		    SELECT user_id, connection_count AS old_count
		    FROM user_presence
		    WHERE user_id = $1
		    FOR UPDATE
		)
		UPDATE user_presence up
		SET connection_count = GREATEST(prev.old_count - 1, 0),
		    disconnected_at = CASE
		        WHEN GREATEST(prev.old_count - 1, 0) = 0 THEN COALESCE(up.disconnected_at, NOW())
		        ELSE NULL
		    END,
		    updated_at = NOW()
		FROM prev
		WHERE up.user_id = prev.user_id
		RETURNING up.connection_count, up.disconnected_at, prev.old_count
	`, userID).Scan(&out.ConnectionCount, &out.DisconnectedAt, &previous)
	if errors.Is(err, sql.ErrNoRows) {
		// A release with no presence row: nothing was ever counted, nothing to do.
		return PresenceTransition{}, nil
	}
	if err != nil {
		return PresenceTransition{}, fmt.Errorf("presence disconnected: %w", err)
	}
	out.Edge = previous == 1 && out.ConnectionCount == 0
	if out.ConnectionCount == 0 {
		// The documents those reports came from are gone with the last socket.
		// Leaving them behind would start the user's next session as away.
		if err := s.clearDocumentVisibilityForUser(ctx, userID); err != nil {
			return PresenceTransition{}, err
		}
	}
	return out, nil
}

// ResetPresenceOnBoot zeroes every socket count, because a fresh process holds no
// sockets. Genuinely-live players re-increment within seconds when their client
// reconnects (retryAttempts: 10), so this is pessimistic in the safe direction:
// Postgres has no TTL, and without this a pod that died without running its defers
// would leave absent players reading as present indefinitely.
//
// COALESCE matters: a restart must not restart the grace window of a player who was
// already inside one.
//
// This is correct at replicas: 1 (k8s/base/backend.yaml). On a scale-out it would
// clear peers' live counts, and the counts themselves are process-agnostic; both need
// per-pod connection ownership instead. They are one decision, not two bugs.
func (s *Store) ResetPresenceOnBoot(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE user_presence
		SET connection_count = 0,
		    disconnected_at = COALESCE(disconnected_at, NOW()),
		    updated_at = NOW()
		WHERE connection_count <> 0 OR disconnected_at IS NULL
	`)
	if err != nil {
		return 0, fmt.Errorf("reset presence on boot: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("reset presence on boot rows affected: %w", err)
	}
	return affected, nil
}
