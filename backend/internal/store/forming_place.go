package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/lfg"
	"github.com/scruffyprodigy/joinquest/internal/lfg/partytree"
)

func (s *Store) tryPlacePartyOnFormingTx(
	ctx context.Context,
	tx *sql.Tx,
	formingMatchID uuid.UUID,
	party *Party,
	partyInput *JoinPartyInput,
) (bool, error) {
	assignments, err := s.ListFormingAssignmentsTx(ctx, tx, formingMatchID)
	if err != nil {
		return false, err
	}
	slots := lfg.SlotsFromAssignments(formingAssignmentsToLFG(assignments))

	tree, err := decodePartyTree(party.PartyTree)
	if err != nil {
		return false, err
	}
	tree = partytree.NormalizeForPlacement(tree)
	if len(tree.AllMembers()) == 0 {
		return false, nil
	}

	seatByUser, ok := partytree.PlaceTree(slots, tree)
	if !ok || len(seatByUser) == 0 {
		return false, nil
	}

	source := "party"
	var tableID *uuid.UUID
	if partyInput != nil && partyInput.TableID != nil {
		source = "table"
		tableID = partyInput.TableID
	} else if tid := strings.TrimSpace(tree.TableID); tid != "" {
		if id, err := uuid.Parse(tid); err == nil {
			source = "table"
			tableID = &id
		}
	} else if len(tree.AllMembers()) == 1 {
		source = "solo"
	}

	return true, s.persistFormingSlotsTx(ctx, tx, formingMatchID, party.ID, seatByUser, source, tableID)
}

func (s *Store) persistFormingSlotsTx(
	ctx context.Context,
	tx *sql.Tx,
	formingMatchID, partyID uuid.UUID,
	seatByUser map[string]string,
	source string,
	tableID *uuid.UUID,
) error {
	for userID, seatKey := range seatByUser {
		uid, err := uuid.Parse(userID)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE forming_match_assignments
			SET user_id = $1, party_id = $2, source = $3, table_id = $4
			WHERE forming_match_id = $5 AND seat_key = $6 AND user_id IS NULL
		`, uid, partyID, source, tableID, formingMatchID, seatKey)
		if err != nil {
			return err
		}
		if err := ensureRowsAffected(result, ErrNotFound); err != nil {
			return fmt.Errorf("store: seat %q already filled", seatKey)
		}
	}
	return nil
}
