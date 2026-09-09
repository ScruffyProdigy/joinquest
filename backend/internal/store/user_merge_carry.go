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

	return carrySourceRatingHistoryTx(ctx, tx, sourceID, targetID)
}

// carrySourceRatingHistoryTx transfers a merged-away user's rating history onto the
// target. Ratings themselves are never merged: a guest and a real account can each hold an
// independent mu/sigma estimate for the same (game, mode), and there is no sound way to
// average two independent skill estimates into one. So instead of merging ratings, this:
//
//  1. Drops every cached rating row (in both player_ratings and nonplayer_ratings) for a
//     (game, mode) that the source's rating history touches. This has to run first, while
//     sourceKey is still present in rating_match_inputs.sides — the DELETE below finds the
//     affected (game_id, mode_key) pairs by searching for sourceKey there, so running it
//     after the rewrite in step 2 (which removes every occurrence of sourceKey) would find
//     nothing to delete.
//  2. Rewrites the source's entrant key to the target's inside rating_match_inputs.sides,
//     so the match history itself — who played whom, and how they placed — survives.
//
// A later replay of the affected modes rebuilds player_ratings/nonplayer_ratings from the
// corrected log (see internal/rating.Replayer), so this is safe by construction: nothing
// here approximates a rating, it only relocates the input that produces one and then
// invalidates the stale cache.
func carrySourceRatingHistoryTx(ctx context.Context, tx *sql.Tx, sourceID, targetID uuid.UUID) error {
	sourceKey := PlayerRatingKey(sourceID)
	targetKey := PlayerRatingKey(targetID)

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM player_ratings
		WHERE (game_id, mode_key) IN (
			SELECT game_id, mode_key FROM rating_match_inputs
			WHERE sides::text LIKE '%' || $1 || '%'
		)
	`, sourceKey); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM nonplayer_ratings
		WHERE (game_id, mode_key) IN (
			SELECT game_id, mode_key FROM rating_match_inputs
			WHERE sides::text LIKE '%' || $1 || '%'
		)
	`, sourceKey); err != nil {
		return err
	}

	// REPLACE on the raw JSON text, not a jsonb-path update: sourceKey is
	// "player:<uuid>", and a UUID string cannot appear as a substring of anything else in
	// this JSON (another entrant's key, a queue-option value, ...), so a blind text
	// substitution cannot corrupt an unrelated field. That is a claim a reviewer should be
	// able to check against the fixed "player:"-prefixed key shape, not take on faith —
	// flagging it here rather than leaving it implicit in the SQL.
	if _, err := tx.ExecContext(ctx, `
		UPDATE rating_match_inputs
		SET sides = REPLACE(sides::text, $1, $2)::jsonb
		WHERE sides::text LIKE '%' || $1 || '%'
	`, sourceKey, targetKey); err != nil {
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
