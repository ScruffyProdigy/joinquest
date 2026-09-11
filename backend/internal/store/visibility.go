package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// SetDocumentVisibility records whether one of the user's open documents is in
// front of them. A document reporting again replaces its own previous answer, so
// a tab flipping between states cannot accumulate rows.
func (s *Store) SetDocumentVisibility(ctx context.Context, userID, documentID uuid.UUID, visible bool) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_document_visibility (document_id, user_id, visible, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (document_id) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    visible = EXCLUDED.visible,
		    updated_at = NOW()
	`, documentID, userID, visible)
	if err != nil {
		return fmt.Errorf("set document visibility: %w", err)
	}
	return nil
}

// UserIsAway reports whether the user holds a live socket with no visible
// document behind it.
//
// Away needs positive evidence on both halves, which is what keeps an unknown
// answer reading as present: the user must be connected, at least one document
// must have reported, and none of the reports may say visible. A user who has
// never reported is present, not away — see the migration for why that default
// is the only safe one.
//
// Deliberately not "disconnected". A user with no sockets is JQ-216's signal and
// returns false here, so a caller can tell the two apart.
func (s *Store) UserIsAway(ctx context.Context, userID uuid.UUID) (bool, error) {
	var away bool
	err := s.db.QueryRowContext(ctx, `SELECT `+userIsAwayExpr, userID).Scan(&away)
	if err != nil {
		return false, fmt.Errorf("user is away: %w", err)
	}
	return away, nil
}

// userIsAwayExpr is the away test as a single boolean expression over $1 = user
// id, so the standalone query above and any caller joining it inside a
// transaction share one definition rather than two that agree by convention.
const userIsAwayExpr = `
	EXISTS (
	    SELECT 1
	    FROM user_presence up
	    WHERE up.user_id = $1
	      AND up.connection_count > 0
	      AND EXISTS (
	          SELECT 1 FROM user_document_visibility d
	          WHERE d.user_id = up.user_id
	      )
	      AND NOT EXISTS (
	          SELECT 1 FROM user_document_visibility d
	          WHERE d.user_id = up.user_id AND d.visible
	      )
	)`

// clearDocumentVisibilityForUser drops every visibility report for a user. Called
// when their last socket closes: the documents behind those reports are gone, and
// a stale hidden row would otherwise make their next session start as away.
func (s *Store) clearDocumentVisibilityForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_document_visibility WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("clear document visibility: %w", err)
	}
	return nil
}

// awayAssignedUnit is one absence on the forming map: the thing a hold window runs
// for, and the thing whose chairs are given up when it is not waited for.
//
// A party is one unit however many chairs it holds (JQ-299). The presence rules are
// written per player, but a party is placed and vacated all-or-nothing, and counting
// a group of three as three absences made "two independent absences" true of a single
// group with two members away -- which vacated chairs that a solo player in the same
// position would have kept, and left the group holding the rest.
type awayAssignedUnit struct {
	// UserID anchors the hold window, which is keyed on a single user. For a party
	// it is the member holding the first seat in seat-key order: stable for as long
	// as the party is on the map, so the next reconcile finds the same anchor and
	// does not restart the window under them.
	//
	// There is no better answer available. A party is away here only when every one
	// of its members is, so there is no present member to notify instead.
	UserID uuid.UUID

	// PartyID is the party the chairs are released by. Every placed seat carries one,
	// a solo player's party of one included; the nil case is defensive.
	PartyID *uuid.UUID
}

// awayAssignedUnitsTx returns the absences on the forming map, in seat order.
//
// A party is away only when every one of its assigned members is away, and counts
// once. Groups are deliberately treated more leniently than solo players: the gate
// exists so nobody is dropped into a game they are not watching, and a group member
// who is present can tell the one who is not -- being in contact with each other is
// what made them a group. A solo player who misses it has nobody to tell them.
//
// Read inside the caller's transaction, which already holds the advisory lock on
// the mode queue, so the answer cannot change under a decision made from it.
func awayAssignedUnitsTx(ctx context.Context, tx *sql.Tx, assignments []FormingAssignment) ([]awayAssignedUnit, error) {
	// A party keys by party so its members collapse into one unit; a seat with no
	// party keys by user, which cannot collide with a party id.
	type unitKey struct{ party, user uuid.UUID }

	awayByUser := make(map[uuid.UUID]bool, len(assignments))
	index := make(map[unitKey]int, len(assignments))
	var units []awayAssignedUnit
	var allAway []bool

	for _, assignment := range assignments {
		if assignment.UserID == nil {
			continue
		}
		userID := *assignment.UserID
		isAway, known := awayByUser[userID]
		if !known {
			var err error
			isAway, err = userIsAwayTx(ctx, tx, userID)
			if err != nil {
				return nil, err
			}
			awayByUser[userID] = isAway
		}

		key := unitKey{user: userID}
		if assignment.PartyID != nil {
			key = unitKey{party: *assignment.PartyID}
		}
		if pos, ok := index[key]; ok {
			// One present member is enough to make the whole party present.
			allAway[pos] = allAway[pos] && isAway
			continue
		}
		index[key] = len(units)
		units = append(units, awayAssignedUnit{UserID: userID, PartyID: assignment.PartyID})
		allAway = append(allAway, isAway)
	}

	var away []awayAssignedUnit
	for i, unit := range units {
		if allAway[i] {
			away = append(away, unit)
		}
	}
	return away, nil
}

// WaitingModeQueueIDForUser reports which mode queue the user is waiting in, if
// any.
//
// Exists so a player coming back to their tab can nudge the forming worker at the
// right queue. Without it a returning player waits out the reconcile tick, which is
// twice the shortest hold window — the chair would still be theirs, but the match
// they were being held for would not start until the poll came round.
func (s *Store) WaitingModeQueueIDForUser(ctx context.Context, userID uuid.UUID) (uuid.UUID, bool, error) {
	var queueID uuid.UUID
	err := s.db.QueryRowContext(ctx, `
		SELECT mode_queue_id
		FROM game_queues
		WHERE user_id = $1 AND status = 'waiting' AND mode_queue_id IS NOT NULL
		ORDER BY joined_at DESC
		LIMIT 1
	`, userID).Scan(&queueID)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("waiting mode queue for user: %w", err)
	}
	return queueID, true, nil
}

// userIsAwayTx is UserIsAway inside a caller's transaction.
func userIsAwayTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (bool, error) {
	var away bool
	if err := tx.QueryRowContext(ctx, `SELECT `+userIsAwayExpr, userID).Scan(&away); err != nil {
		return false, fmt.Errorf("user is away: %w", err)
	}
	return away, nil
}
