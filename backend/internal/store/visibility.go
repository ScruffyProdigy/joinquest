package store

import (
	"context"
	"database/sql"
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

// awayAssignedUsersTx returns the distinct assigned players who hold a live socket
// with nothing visible behind it, in assignment order.
//
// Read inside the caller's transaction, which already holds the advisory lock on
// the mode queue, so the answer cannot change under a decision made from it.
func awayAssignedUsersTx(ctx context.Context, tx *sql.Tx, assignments []FormingAssignment) ([]uuid.UUID, error) {
	seen := make(map[uuid.UUID]struct{}, len(assignments))
	var away []uuid.UUID
	for _, assignment := range assignments {
		if assignment.UserID == nil {
			continue
		}
		userID := *assignment.UserID
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}

		var isAway bool
		if err := tx.QueryRowContext(ctx, `SELECT `+userIsAwayExpr, userID).Scan(&isAway); err != nil {
			return nil, fmt.Errorf("away assigned users: %w", err)
		}
		if isAway {
			away = append(away, userID)
		}
	}
	return away, nil
}
