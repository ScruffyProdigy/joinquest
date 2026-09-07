package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/lfg/partytree"
)

func (s *Store) getPartyByIDTx(ctx context.Context, tx *sql.Tx, partyID uuid.UUID) (*Party, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, leader_user_id, mode_queue_id, status, party_tree, created_at
		FROM parties
		WHERE id = $1
	`, partyID)
	return scanPartyRow(row)
}

func listPartyMembersTx(ctx context.Context, tx *sql.Tx, partyID uuid.UUID) ([]JoinPartyMemberInput, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT user_id, queue_path
		FROM party_members
		WHERE party_id = $1
		ORDER BY sort_order ASC
	`, partyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []JoinPartyMemberInput
	for rows.Next() {
		var member JoinPartyMemberInput
		if err := rows.Scan(&member.UserID, &member.QueuePath); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func partyAssignedOnFormingTx(ctx context.Context, tx *sql.Tx, formingMatchID, partyID uuid.UUID) (bool, error) {
	var one int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM forming_match_assignments
		WHERE forming_match_id = $1 AND party_id = $2 AND user_id IS NOT NULL
		LIMIT 1
	`, formingMatchID, partyID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// syncWaitingPartiesOnFormingTx places every waiting party that is not yet on the map.
// Called on every join so idempotent re-joins heal missed placements.
func (s *Store) syncWaitingPartiesOnFormingTx(
	ctx context.Context,
	tx *sql.Tx,
	fm *FormingMatch,
	modeQueueID uuid.UUID,
) error {
	entries, err := listWaitingModeQueueEntriesTx(ctx, tx, modeQueueID)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.PartyID == nil {
			continue
		}
		assigned, err := partyAssignedOnFormingTx(ctx, tx, fm.ID, *entry.PartyID)
		if err != nil {
			return err
		}
		if assigned {
			continue
		}

		party, err := s.getPartyByIDTx(ctx, tx, *entry.PartyID)
		if err != nil {
			return err
		}
		members, err := listPartyMembersTx(ctx, tx, party.ID)
		if err != nil {
			return err
		}
		if len(members) == 0 {
			continue
		}

		tree, err := decodePartyTree(party.PartyTree)
		if err != nil {
			return err
		}
		tree = partytree.NormalizeForPlacement(tree)
		input := &JoinPartyInput{Tree: tree, Members: members}

		placed, err := s.tryPlacePartyOnFormingTx(ctx, tx, fm.ID, party, input)
		if err != nil {
			return err
		}
		if !placed {
			continue
		}
		if err := s.markPartyStatusTx(ctx, tx, party.ID, PartyStatusPlaced); err != nil {
			return err
		}
		if err := s.linkPartyQueuesToFormingTx(ctx, tx, party.ID, fm.ID); err != nil {
			return err
		}
	}
	return nil
}
