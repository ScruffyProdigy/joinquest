package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/prequeue"
)

type formingFireResult struct {
	session   *Session
	notifyIDs []uuid.UUID
	tableIDs  []uuid.UUID
}

func (s *Store) joinModeQueueFormingTx(
	ctx context.Context,
	tx *sql.Tx,
	joinCtx *ModeQueueJoinContext,
	modeQueueID, callerID uuid.UUID,
	queuePath string,
	options []prequeue.Selection,
	partyInput *JoinPartyInput,
) (*QueueJoinResult, error) {
	var members []JoinPartyMemberInput
	var enqueue enqueueOutcome
	skipPlacement := false

	if partyInput != nil && len(partyInput.Members) > 0 {
		members = partyInput.Members
		// The caller's own picks arrive on the mutation rather than inside the
		// party tree; every other member answers for themselves.
		for i := range members {
			if members[i].UserID == callerID && len(members[i].QueueOptions) == 0 {
				members[i].QueueOptions = options
			}
		}
	} else {
		members = []JoinPartyMemberInput{{UserID: callerID, QueuePath: queuePath, QueueOptions: options}}
	}

	if existing, err := getWaitingQueueEntryForUserTx(ctx, tx, modeQueueID, callerID); err == nil {
		if sameQueuePath(existing.QueuePath, queuePath) {
			skipPlacement = true
			enqueue.alreadyInQueue = true
		}
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	if !skipPlacement {
		for _, member := range members {
			if err := s.prepareCatalogPartyJoinTx(ctx, tx, member.UserID); err != nil {
				return nil, err
			}
		}
		if partyInput != nil && len(partyInput.Members) > 0 {
			tree := partyInput.Tree
			if len(tree.AllMembers()) == 0 {
				return nil, fmt.Errorf("store: party tree is required")
			}
			created, err := s.CreatePartyFromTreeTx(ctx, tx, modeQueueID, callerID, tree, members)
			if err != nil {
				return nil, err
			}
			partyInput.Tree = tree
			for _, member := range members {
				out, err := enqueueModeQueueTx(ctx, tx, joinCtx.Game.ID, modeQueueID, member.UserID, member.QueuePath, member.QueueOptions, &created.ID)
				if err != nil {
					return nil, err
				}
				if member.UserID == callerID {
					enqueue = out
				}
			}
		} else {
			created, err := s.CreateSoloPartyTx(ctx, tx, modeQueueID, callerID, SoloPartyInput{
				QueuePath: queuePath,
			})
			if err != nil {
				return nil, err
			}
			enqueue, err = enqueueModeQueueTx(ctx, tx, joinCtx.Game.ID, modeQueueID, callerID, queuePath, options, &created.ID)
			if err != nil {
				return nil, err
			}
		}
	}

	waiting, err := listWaitingModeQueueEntriesTx(ctx, tx, modeQueueID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &QueueJoinResult{
		GameID:         joinCtx.Game.ID,
		ModeQueueID:    modeQueueID,
		Status:         QueueStatusWaiting,
		QueuedCount:    len(waiting),
		QueuePath:      optionalQueuePathRef(queuePath),
		NotifyUserIDs:  notifyUserIDsFromMembers(members),
		AlreadyInQueue: enqueue.alreadyInQueue,
		SwitchedFrom:   enqueue.switchedFrom,
	}, nil
}

func notifyUserIDsFromMembers(members []JoinPartyMemberInput) []uuid.UUID {
	ids := make([]uuid.UUID, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	return ids
}

func (s *Store) markPartyStatusTx(ctx context.Context, tx *sql.Tx, partyID uuid.UUID, status string) error {
	_, err := tx.ExecContext(ctx, `UPDATE parties SET status = $2 WHERE id = $1`, partyID, status)
	return err
}

func (s *Store) linkPartyQueuesToFormingTx(ctx context.Context, tx *sql.Tx, partyID, formingMatchID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE game_queues
		SET forming_match_id = $1
		WHERE party_id = $2 AND status = 'waiting'
	`, formingMatchID, partyID)
	return err
}

func (s *Store) fireFormingMatchTx(
	ctx context.Context,
	tx *sql.Tx,
	joinCtx *ModeQueueJoinContext,
	fm *FormingMatch,
) (*formingFireResult, error) {
	assignments, err := s.ListFormingAssignmentsTx(ctx, tx, fm.ID)
	if err != nil {
		return nil, err
	}

	// Resolve every assigned player's waiting row BEFORE anything is written, because
	// one of them may no longer have one. Every removal path is supposed to release
	// the player's assignment in the same transaction as the cancel, but a path that
	// forgets must not be able to wedge the whole mode queue: a hard error here rolls
	// back the reconcile, and since nothing expires a filling forming match, it rolls
	// back every future reconcile too. So vacate the orphaned seat and decline to
	// fire — the next reconcile refills it from the waiting pool.
	entries := make(map[uuid.UUID]*QueueEntry, len(assignments))
	vacated := false
	for _, assignment := range assignments {
		if assignment.UserID == nil {
			continue
		}
		userID := *assignment.UserID
		if _, ok := entries[userID]; ok {
			continue
		}
		entry, err := getWaitingQueueEntryForUserTx(ctx, tx, joinCtx.ModeQueue.ID, userID)
		if errors.Is(err, ErrNotFound) {
			if err := s.releaseFormingSlotsForUserTx(ctx, tx, userID); err != nil {
				return nil, err
			}
			vacated = true
			continue
		}
		if err != nil {
			return nil, err
		}
		entries[userID] = entry
	}
	if vacated {
		// Declining is not an error: the caller commits the vacate and reports an
		// unfired reconcile, exactly as it would for a match that is merely still
		// short of players.
		return nil, nil
	}

	session, err := createModeQueueSessionTx(ctx, tx, joinCtx.Game.ID, joinCtx.Mode.ID, joinCtx.ModeQueue.ID)
	if err != nil {
		return nil, err
	}
	// Deliberately no "complete every other active session on this queue" step here.
	// mode_queues is catalog configuration reused by every match on a mode, so a step
	// like that ended other players' live games (JQ-171). Nothing needs it: a player's
	// own stale participation is already resolved per-user, by JoinModeQueue's
	// finishReturnedQueueSessionsForRequeueTx and completeEmptiedSessionsForUserTx, by
	// AcknowledgePlayerReturn, and by GetMatchedSessionForUserAndModeQueue joining
	// through this user's own participant row.

	notifyIDs := make([]uuid.UUID, 0, len(assignments))
	tableIDs := make([]uuid.UUID, 0)
	seen := make(map[uuid.UUID]struct{})
	tableSeen := make(map[uuid.UUID]struct{})
	partySeen := make(map[uuid.UUID]struct{})

	for _, assignment := range assignments {
		if assignment.UserID == nil {
			continue
		}
		userID := *assignment.UserID
		if _, ok := seen[userID]; !ok {
			// One row per player, even in the shouldn't-happen case of a player
			// holding two seats: the pre-pass resolved their waiting row once, and
			// marking it matched twice would add them to the session twice.
			seen[userID] = struct{}{}
			notifyIDs = append(notifyIDs, userID)

			entry := entries[userID]
			if err := markQueueEntryMatchedTx(ctx, tx, entry.ID); err != nil {
				return nil, err
			}
			returnCtx := CatalogLFGReturnContext(joinCtx.Game.ID, joinCtx.ModeQueue.ID)
			if err := addSessionParticipantTx(ctx, tx, session.ID, userID, assignment.SeatKey, returnCtx, entry.QueueOptions); err != nil {
				return nil, err
			}
		}
		if assignment.PartyID != nil {
			if _, ok := partySeen[*assignment.PartyID]; !ok {
				partySeen[*assignment.PartyID] = struct{}{}
				if err := s.markPartyStatusTx(ctx, tx, *assignment.PartyID, PartyStatusMatched); err != nil {
					return nil, err
				}
			}
		}
		if assignment.TableID != nil {
			if _, ok := tableSeen[*assignment.TableID]; !ok {
				tableSeen[*assignment.TableID] = struct{}{}
				tableIDs = append(tableIDs, *assignment.TableID)
			}
		}
	}

	if err := s.MarkFormingMatchFiredTx(ctx, tx, fm.ID, time.Now()); err != nil {
		return nil, err
	}

	return &formingFireResult{session: session, notifyIDs: notifyIDs, tableIDs: tableIDs}, nil
}

func getWaitingQueueEntryForUserTx(ctx context.Context, tx *sql.Tx, modeQueueID, userID uuid.UUID) (*QueueEntry, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT `+queueColumns+`
		FROM game_queues
		WHERE mode_queue_id = $1 AND user_id = $2 AND status = 'waiting'
		ORDER BY joined_at DESC
		LIMIT 1
	`, modeQueueID, userID)
	return scanQueueEntry(row)
}
