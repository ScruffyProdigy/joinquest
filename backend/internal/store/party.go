package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/lfg/partytree"
)

// Catalog play intent lives on game_queues (status = waiting). Parties are forming-engine
// metadata linked through game_queues.party_id — not a separate player-visible state.

// SoloPartyInput describes a single-player party for queue join.
type SoloPartyInput struct {
	QueuePath string
}

// JoinPartyMemberInput is one member in a queue join party.
type JoinPartyMemberInput struct {
	UserID    uuid.UUID
	QueuePath string
}

// JoinPartyInput describes a party joining a mode queue.
type JoinPartyInput struct {
	Tree    partytree.Node
	Members []JoinPartyMemberInput
	Pinned  []partytree.PinnedSeat
	TableID *uuid.UUID
}

// CreatePartyFromTreeTx creates a party with a layout tree and member rows.
func (s *Store) CreatePartyFromTreeTx(
	ctx context.Context,
	tx *sql.Tx,
	modeQueueID uuid.UUID,
	leaderID uuid.UUID,
	tree partytree.Node,
	members []JoinPartyMemberInput,
) (*Party, error) {
	if len(members) == 0 {
		return nil, fmt.Errorf("store: party requires at least one member")
	}
	for _, member := range members {
		active, err := s.getActivePartyForUserTx(ctx, tx, member.UserID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		if active != nil {
			return nil, fmt.Errorf("store: user %s already in an active party", member.UserID)
		}
	}

	treeJSON, err := json.Marshal(tree)
	if err != nil {
		return nil, fmt.Errorf("store: encode party tree: %w", err)
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO parties (leader_user_id, mode_queue_id, status, party_tree)
		VALUES ($1, $2, $3, $4)
		RETURNING id, leader_user_id, mode_queue_id, status, party_tree, created_at
	`, leaderID, modeQueueID, PartyStatusWaiting, treeJSON)
	party, err := scanPartyRow(row)
	if err != nil {
		return nil, err
	}

	for i, member := range members {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO party_members (party_id, user_id, queue_path, sort_order)
			VALUES ($1, $2, $3, $4)
		`, party.ID, member.UserID, strings.TrimSpace(member.QueuePath), i); err != nil {
			return nil, err
		}
	}
	return party, nil
}

// CreateSoloPartyTx creates a one-member party for queue join.
func (s *Store) CreateSoloPartyTx(
	ctx context.Context,
	tx *sql.Tx,
	modeQueueID, userID uuid.UUID,
	input SoloPartyInput,
) (*Party, error) {
	tree := partytree.SoloNode(userID.String(), input.QueuePath)
	return s.CreatePartyFromTreeTx(ctx, tx, modeQueueID, userID, tree, []JoinPartyMemberInput{{
		UserID:    userID,
		QueuePath: input.QueuePath,
	}})
}

func scanPartyRow(row interface{ Scan(dest ...any) error }) (*Party, error) {
	var party Party
	var leader sql.NullString
	var treeJSON []byte
	if err := row.Scan(
		&party.ID, &leader, &party.ModeQueueID, &party.Status, &treeJSON, &party.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if leader.Valid {
		id, err := uuid.Parse(leader.String)
		if err != nil {
			return nil, err
		}
		party.LeaderUserID = &id
	}
	if len(treeJSON) > 0 {
		party.PartyTree = treeJSON
	}
	return &party, nil
}

// getActivePartyForUserTx returns the party tied to the user's waiting queue row, if any.
func (s *Store) getActivePartyForUserTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (*Party, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT p.id, p.leader_user_id, p.mode_queue_id, p.status, p.party_tree, p.created_at
		FROM game_queues gq
		INNER JOIN parties p ON p.id = gq.party_id
		WHERE gq.user_id = $1
		  AND gq.status = 'waiting'
		  AND gq.party_id IS NOT NULL
		  AND p.status IN ($2, $3)
		ORDER BY gq.joined_at DESC
		LIMIT 1
	`, userID, PartyStatusWaiting, PartyStatusPlaced)
	return scanPartyRow(row)
}

// CancelPartyTx marks a party cancelled.
func (s *Store) CancelPartyTx(ctx context.Context, tx *sql.Tx, partyID uuid.UUID) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE parties SET status = $2 WHERE id = $1 AND status IN ($3, $4)
	`, partyID, PartyStatusCancelled, PartyStatusWaiting, PartyStatusPlaced)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, ErrNotFound)
}

// prepareCatalogPartyJoinTx clears parties being replaced and garbage-collects stale rows.
func (s *Store) prepareCatalogPartyJoinTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	if err := s.reconcileStalePartiesForUserTx(ctx, tx, userID); err != nil {
		return err
	}
	return s.cancelPartyForWaitingUserTx(ctx, tx, userID)
}

func (s *Store) reconcileStalePartiesForUser(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.reconcileStalePartiesForUserTx(ctx, tx, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) reconcileStalePartiesForUserTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT p.id
		FROM parties p
		INNER JOIN party_members pm ON pm.party_id = p.id
		WHERE pm.user_id = $1
		  AND p.status IN ($2, $3)
		  AND NOT EXISTS (
		    SELECT 1
		    FROM game_queues gq
		    WHERE gq.party_id = p.id AND gq.status = 'waiting'
		  )
	`, userID, PartyStatusWaiting, PartyStatusPlaced)
	if err != nil {
		return err
	}

	var partyIDs []uuid.UUID
	for rows.Next() {
		var partyID uuid.UUID
		if err := rows.Scan(&partyID); err != nil {
			rows.Close()
			return err
		}
		partyIDs = append(partyIDs, partyID)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, partyID := range partyIDs {
		if err := s.cancelPartyTx(ctx, tx, partyID, userID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) cancelPartyForWaitingUserTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	row := tx.QueryRowContext(ctx, `
		SELECT party_id
		FROM game_queues
		WHERE user_id = $1 AND status = 'waiting' AND party_id IS NOT NULL
		ORDER BY joined_at DESC
		LIMIT 1
	`, userID)
	var partyID uuid.UUID
	if err := row.Scan(&partyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	return s.cancelPartyTx(ctx, tx, partyID, userID)
}

func (s *Store) cancelPartyTx(ctx context.Context, tx *sql.Tx, partyID, userID uuid.UUID) error {
	if err := s.releasePartyFormingSlotsTx(ctx, tx, partyID); err != nil {
		return err
	}
	if err := s.releaseFormingSlotsForUserTx(ctx, tx, userID); err != nil {
		return err
	}
	if err := s.CancelPartyTx(ctx, tx, partyID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE game_queues
		SET party_id = NULL, forming_match_id = NULL
		WHERE party_id = $1 AND status = 'waiting'
	`, partyID)
	return err
}

func (s *Store) releasePartyFormingSlotsTx(ctx context.Context, tx *sql.Tx, partyID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE forming_match_assignments
		SET user_id = NULL, party_id = NULL, source = 'solo', table_id = NULL
		WHERE party_id = $1
	`, partyID)
	return err
}

func (s *Store) releaseFormingSlotsForUserTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE forming_match_assignments
		SET user_id = NULL, party_id = NULL, source = 'solo', table_id = NULL
		WHERE user_id = $1
	`, userID)
	return err
}

func decodePartyTree(raw json.RawMessage) (partytree.Node, error) {
	if len(raw) == 0 {
		return partytree.Node{}, nil
	}
	var tree partytree.Node
	if err := json.Unmarshal(raw, &tree); err != nil {
		return partytree.Node{}, fmt.Errorf("store: decode party tree: %w", err)
	}
	return tree, nil
}
