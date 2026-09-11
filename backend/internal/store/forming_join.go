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

	// Every chair is filled, but a filled chair is not an attentive player. Firing
	// now would drop someone into a game they are not looking at, and the seat is
	// only recoverable before this point -- once the session exists there is no
	// chair left to put a replacement in.
	away, err := awayAssignedUsersTx(ctx, tx, assignments)
	if err != nil {
		return nil, err
	}
	switch {
	case len(away) > 1:
		// Never hold two chairs at once. One held chair cannot disappoint anybody:
		// the held player's return is itself the fire condition. Two can — tell both
		// to come back, one does, and they arrive to a match that never formed.
		//
		// But a window already running has very likely been announced to that player
		// ("come back now to keep your spot"), and vacating them because somebody
		// else then wandered off would make that message retroactively false. So the
		// running hold stands and only the new absences give up their chairs.
		holding, err := heldUserIDTx(ctx, tx, fm.ID)
		if err != nil {
			return nil, err
		}
		kept := false
		for _, userID := range away {
			if holding != nil && *holding == userID {
				kept = true
				continue
			}
			if err := s.releaseFormingSlotsForUserTx(ctx, tx, userID); err != nil {
				return nil, err
			}
		}
		if !kept {
			// Nobody had been promised anything yet, so there is no window to keep.
			if err := clearHoldTx(ctx, tx, fm.ID); err != nil {
				return nil, err
			}
		}
		return nil, nil

	case len(away) == 1:
		expired, err := advanceHoldTx(ctx, tx, fm.ID, away[0], holdWindowFor(false))
		if err != nil {
			return nil, err
		}
		if !expired {
			// Declining leaves the assignment intact, so the held chair survives to
			// the next reconcile. Nobody has been told a match formed, so the players
			// who are present are still simply queuing rather than watching a stall.
			return nil, nil
		}
		// Out of time. Vacating the chair is not ejecting the player: their waiting
		// row is untouched, so they stay in line for the next table. They were never
		// told a match formed, so there is nothing to explain to them.
		if err := s.releaseFormingSlotsForUserTx(ctx, tx, away[0]); err != nil {
			return nil, err
		}
		if err := clearHoldTx(ctx, tx, fm.ID); err != nil {
			return nil, err
		}
		return nil, nil
	}

	// Everyone is here. Drop any window left over from an absence that resolved,
	// so the next one starts from scratch rather than inheriting a stale stamp.
	if err := clearHoldTx(ctx, tx, fm.ID); err != nil {
		return nil, err
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
	// One lookup per table rather than per player: a six-seat match formed from two
	// backfilling tables asks twice, not six times.
	returnCtxByTable := make(map[uuid.UUID]ReturnContext)

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
			returnCtx, err := formingReturnContextTx(ctx, tx, joinCtx, assignment.TableID, returnCtxByTable)
			if err != nil {
				return nil, err
			}
			if err := addSessionParticipantTx(ctx, tx, joinCtx.Mode, session.ID, userID, assignment.SeatKey, returnCtx, entry.QueueOptions); err != nil {
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

// formingReturnContextTx answers "where did this player come from" for a matchmade session:
// the room table they were sitting at, or the catalog queue when they joined on their own.
//
// This is the only record of who arrived with whom that survives the fire. A table that
// backfills goes through matchmaking as a party, and both `forming_match_assignments.party_id`
// and `.table_id` belong to the forming map, which is finished with the moment this commits —
// nothing copies them onto the participant row. Stamping the room here is what lets the
// post-match screen partition six players back into the two groups that arrived (JQ-291),
// and `return_context` is the right place for it because it is written once at session start
// and never rewritten: room_tables.session_id is cleared by the post-match reset, and
// game_sessions.regroup_table_id is stamped by whichever player claims first.
//
// A vanished table is not an error. Nothing stops a room closing between the backfill and the
// fire, and a lost arrival room costs the player their group's regroup card — worth far less
// than failing the match everyone is waiting on. They fall back to arriving alone.
func formingReturnContextTx(
	ctx context.Context,
	tx *sql.Tx,
	joinCtx *ModeQueueJoinContext,
	tableID *uuid.UUID,
	cache map[uuid.UUID]ReturnContext,
) (ReturnContext, error) {
	lfg := CatalogLFGReturnContext(joinCtx.Game.ID, joinCtx.ModeQueue.ID)
	if tableID == nil {
		return lfg, nil
	}
	if cached, ok := cache[*tableID]; ok {
		return cached, nil
	}

	var (
		roomID     uuid.UUID
		inviteCode string
	)
	err := tx.QueryRowContext(ctx, `
		SELECT t.room_id, r.invite_code
		FROM room_tables t
		INNER JOIN rooms r ON r.id = t.room_id
		WHERE t.id = $1 AND r.status = 'open'
	`, *tableID).Scan(&roomID, &inviteCode)
	if errors.Is(err, sql.ErrNoRows) {
		cache[*tableID] = lfg
		return lfg, nil
	}
	if err != nil {
		return ReturnContext{}, err
	}

	returnCtx := RoomTableReturnContext(inviteCode, joinCtx.Game.ID, roomID, *tableID)
	cache[*tableID] = returnCtx
	return returnCtx, nil
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
