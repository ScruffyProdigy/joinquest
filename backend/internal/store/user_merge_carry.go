package store

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// carryUserScopedRowsTx moves every row keyed on the source user onto the target, in the
// order the constraints demand: unwind the source's live intent first so its rows are
// terminal, then move history, then ownership, then discard credentials.
//
// Ordering is the mechanism, not an implementation detail. Several of these tables carry
// uniqueness constraints that a plain UPDATE ... SET user_id = target violates when both
// accounts are live at once — idx_game_queues_one_waiting_per_user is global, and
// room_members and table_seats are unique per user. Unwinding first means nothing that
// crosses over can collide with a row the target already holds.
func (s *Store) carryUserScopedRowsTx(ctx context.Context, tx *sql.Tx, sourceID, targetID uuid.UUID) error {
	if err := s.unwindSourceLiveIntentTx(ctx, tx, sourceID); err != nil {
		return err
	}
	if err := carrySourceHistoryTx(ctx, tx, sourceID, targetID); err != nil {
		return err
	}
	if err := carrySourceOwnershipTx(ctx, tx, sourceID, targetID); err != nil {
		return err
	}
	return discardSourceCredentialsTx(ctx, tx, sourceID)
}

// unwindSourceLiveIntentTx resolves the source's live state to one coherent, terminal
// shape. The target's live state is deliberately untouched: the target is the account the
// player is actually signed into, so it wins every conflict.
func (s *Store) unwindSourceLiveIntentTx(ctx context.Context, tx *sql.Tx, sourceID uuid.UUID) error {
	// A live seat's launch URL was minted by the game against the source user id, so the
	// target could never authenticate into it. Retire it as history instead.
	if _, err := tx.ExecContext(ctx, `
		UPDATE game_session_participants gsp
		SET finished_at = NOW()
		FROM game_sessions gs
		WHERE gsp.session_id = gs.id
		  AND gsp.user_id = $1
		  AND gsp.left_at IS NULL
		  AND gsp.finished_at IS NULL
		  AND gs.status = 'active'
	`, sourceID); err != nil {
		return err
	}

	// Parties are read off the source's still-waiting queue rows, so this has to run
	// before those rows are cancelled below.
	if err := s.cancelPartyForWaitingUserTx(ctx, tx, sourceID); err != nil {
		return err
	}
	if err := s.releaseFormingSlotsForUserTx(ctx, tx, sourceID); err != nil {
		return err
	}
	if err := leaveUserWaitingQueuesTx(ctx, tx, sourceID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE game_queues SET status = 'cancelled'
		WHERE user_id = $1 AND status = 'matched'
	`, sourceID); err != nil {
		return err
	}

	if _, _, err := s.leaveTableSeatTx(ctx, tx, sourceID); err != nil {
		return err
	}
	// leaveTableSeatTx only releases seats at forming tables; table_seats is unique per
	// user, so anything left at a started table has to go too.
	if _, err := tx.ExecContext(ctx, `DELETE FROM table_seats WHERE user_id = $1`, sourceID); err != nil {
		return err
	}

	// Handles host reassignment and closing a room the source was alone in.
	if _, err := s.leaveRoomTx(ctx, tx, sourceID); err != nil {
		return err
	}
	return nil
}

// carrySourceHistoryTx moves what the player did. Every row here is terminal by the time
// it runs, so the only conflicts left are rows both accounts hold in the same parent.
func carrySourceHistoryTx(ctx context.Context, tx *sql.Tx, sourceID, targetID uuid.UUID) error {
	// UNIQUE(session_id, user_id): if both accounts sat in one match, the target's row
	// wins and the source's is dropped.
	if _, err := tx.ExecContext(ctx, `
		UPDATE game_session_participants
		SET user_id = $2
		WHERE user_id = $1
		  AND NOT EXISTS (
			SELECT 1 FROM game_session_participants existing
			WHERE existing.session_id = game_session_participants.session_id
			  AND existing.user_id = $2
		  )
	`, sourceID, targetID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM game_session_participants WHERE user_id = $1
	`, sourceID); err != nil {
		return err
	}

	// game_sessions has no user column; it follows participation.

	// Safe unguarded: the unwind left the source with no waiting rows, and the unique
	// index is partial on status = 'waiting'.
	if _, err := tx.ExecContext(ctx, `
		UPDATE game_queues SET user_id = $2 WHERE user_id = $1
	`, sourceID, targetID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE room_messages SET user_id = $2 WHERE user_id = $1
	`, sourceID, targetID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE avatar_readings SET user_id = $2 WHERE user_id = $1
	`, sourceID, targetID); err != nil {
		return err
	}

	// UNIQUE(party_id, user_id).
	if _, err := tx.ExecContext(ctx, `
		UPDATE party_members
		SET user_id = $2
		WHERE user_id = $1
		  AND NOT EXISTS (
			SELECT 1 FROM party_members existing
			WHERE existing.party_id = party_members.party_id
			  AND existing.user_id = $2
		  )
	`, sourceID, targetID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM party_members WHERE user_id = $1`, sourceID); err != nil {
		return err
	}
	return nil
}

// carrySourceOwnershipTx moves what the player owns. None of these are constrained per
// user, so they move outright.
func carrySourceOwnershipTx(ctx context.Context, tx *sql.Tx, sourceID, targetID uuid.UUID) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE games SET owner_user_id = $2 WHERE owner_user_id = $1
	`, sourceID, targetID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE developer_api_keys SET user_id = $2 WHERE user_id = $1
	`, sourceID, targetID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE rooms SET host_user_id = $2 WHERE host_user_id = $1
	`, sourceID, targetID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE parties SET leader_user_id = $2 WHERE leader_user_id = $1
	`, sourceID, targetID); err != nil {
		return err
	}
	return nil
}

// discardSourceCredentialsTx destroys what must not survive. A magic link is a live login
// token; handing one to the survivor, or leaving it usable against an account about to be
// deactivated, is strictly worse than losing it.
func discardSourceCredentialsTx(ctx context.Context, tx *sql.Tx, sourceID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM magic_links WHERE user_id = $1`, sourceID)
	return err
}
