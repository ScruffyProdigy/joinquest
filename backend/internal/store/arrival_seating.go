package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

// autoSeatArrivalTx seats a player at the table they have just created or joined, when
// the mode leaves them nothing to decide (JQ-306).
//
// It is the arrival half of the rule rejoining players already get: where there is no
// choice to make, making somebody click an identical seat is friction with no upside.
// ModeOffersPreMatchChoice is the same trigger both halves read, so "no decision needed"
// means one thing across the product rather than two things that drift apart.
//
// Declining to seat is never an error. Every reason to decline — the mode asks for a
// pick, the table is full, the player is mid-match or already matched into a queue —
// leaves the arrival exactly where arrivals landed before this existed: unseated, in
// Picking a seat, free to claim. An arrival must not fail because a convenience could
// not be applied, so only a genuine database fault propagates.
//
// Reports whether the player ended up seated by this call.
func (s *Store) autoSeatArrivalTx(ctx context.Context, tx *sql.Tx, table *RoomTable, userID uuid.UUID) (bool, error) {
	if table == nil || table.Status != TableStatusForming {
		return false, nil
	}

	mode, err := getGameModeByID(ctx, tx, table.ModeID)
	if err != nil {
		return false, err
	}
	if ModeOffersPreMatchChoice(mode) {
		return false, nil
	}

	// A player already matched into a queue, or inside a live session, has a seat
	// elsewhere that this one would silently take them out of. sitAtTableTx refuses
	// both; ask first so the refusal does not fail the arrival itself.
	if err := ensureNotQueueMatchedTx(ctx, tx, userID); err != nil {
		if errors.Is(err, ErrAlreadyMatched) {
			return false, nil
		}
		return false, err
	}
	if err := ensureNotInActiveGameTx(ctx, tx, userID); err != nil {
		if errors.Is(err, ErrActiveGame) {
			return false, nil
		}
		return false, err
	}

	seatKey, err := s.openSeatKeyTx(ctx, tx, table, userID, "")
	if err != nil {
		if errors.Is(err, ErrTableFull) {
			return false, nil
		}
		return false, err
	}
	if seatKey == "" {
		return false, nil
	}

	if _, err := s.sitAtTableTx(ctx, tx, table.ID, userID, seatKey, nil); err != nil {
		return false, err
	}
	return true, nil
}

// soleFormingTableTx returns the room's only forming table, or nil when the room has
// none or has more than one.
//
// More than one is itself a decision — which table to sit at — so an arrival into a
// room of several tables chooses, exactly as they do today. It is also the condition
// the client already uses to send an arrival straight to /group (lib/group.js's
// groupLandingPath), so the two agree on when a room has an obvious destination.
func soleFormingTableTx(ctx context.Context, tx *sql.Tx, roomID uuid.UUID) (*RoomTable, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT `+roomTableColumns+`
		FROM room_tables
		WHERE room_id = $1 AND status = $2
		ORDER BY created_at ASC
		LIMIT 2
	`, roomID, TableStatusForming)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []RoomTable
	for rows.Next() {
		table, err := scanRoomTable(rows)
		if err != nil {
			return nil, err
		}
		tables = append(tables, *table)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(tables) != 1 {
		return nil, nil
	}
	return &tables[0], nil
}
